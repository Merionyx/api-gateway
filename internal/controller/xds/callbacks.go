package xds

import (
	"context"
	"log/slog"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"

	ctrlmetrics "github.com/merionyx/api-gateway/internal/controller/metrics"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"
)

// spanXDSPkg is the package path for optional xDS callback spans.
const spanXDSPkg = "internal/controller/xds"

// Callbacks implements go-control-plane server callbacks with Prometheus metrics.
type Callbacks struct {
	enabled      bool
	traceXDS     bool
}

func NewCallbacks(metricsEnabled bool, xdsTraceCallbacks bool) *Callbacks {
	return &Callbacks{enabled: metricsEnabled, traceXDS: xdsTraceCallbacks}
}

func (c *Callbacks) OnStreamOpen(ctx context.Context, streamID int64, typeURL string) error {
	slog.Debug("xDS stream opened", "stream_id", streamID, "type_url", typeURL)
	ctrlmetrics.XDSStreamOpened(c.enabled)
	return nil
}

func (c *Callbacks) OnStreamClosed(streamID int64, node *corev3.Node) {
	slog.Debug("xDS stream closed", "stream_id", streamID)
	ctrlmetrics.XDSStreamClosed(c.enabled)
}

func (c *Callbacks) OnStreamRequest(streamID int64, req *discoveryv3.DiscoveryRequest) error {
	if c.traceXDS {
		_, sp := telemetry.Start(context.Background(), telemetry.SpanName(spanXDSPkg, "OnStreamRequest"))
		defer sp.End()
	}
	slog.Debug("xDS stream request",
		"stream_id", streamID, "node", req.GetNode().GetId(), "version", req.GetVersionInfo())
	ctrlmetrics.RecordXDSStreamRequest(c.enabled, req.GetTypeUrl())
	return nil
}

func (c *Callbacks) OnStreamResponse(ctx context.Context, streamID int64, req *discoveryv3.DiscoveryRequest, resp *discoveryv3.DiscoveryResponse) {
	slog.Debug("xDS stream response",
		"stream_id", streamID, "type_url", resp.GetTypeUrl(), "version", resp.GetVersionInfo())
}

func (c *Callbacks) OnFetchRequest(ctx context.Context, req *discoveryv3.DiscoveryRequest) error {
	if c.traceXDS {
		_, sp := telemetry.Start(ctx, telemetry.SpanName(spanXDSPkg, "OnFetchRequest"))
		defer sp.End()
	}
	ctrlmetrics.RecordXDSStreamRequest(c.enabled, req.GetTypeUrl())
	return nil
}

func (c *Callbacks) OnFetchResponse(req *discoveryv3.DiscoveryRequest, resp *discoveryv3.DiscoveryResponse) {
}

// Delta xDS methods
func (c *Callbacks) OnDeltaStreamOpen(ctx context.Context, streamID int64, typeURL string) error {
	slog.Debug("xDS delta stream opened", "stream_id", streamID, "type_url", typeURL)
	ctrlmetrics.XDSStreamOpened(c.enabled)
	return nil
}

func (c *Callbacks) OnDeltaStreamClosed(streamID int64, node *corev3.Node) {
	slog.Debug("xDS delta stream closed", "stream_id", streamID)
	ctrlmetrics.XDSStreamClosed(c.enabled)
}

func (c *Callbacks) OnStreamDeltaRequest(streamID int64, req *discoveryv3.DeltaDiscoveryRequest) error {
	slog.Debug("xDS delta stream request", "stream_id", streamID, "node", req.GetNode().GetId())
	ctrlmetrics.RecordXDSStreamRequest(c.enabled, req.GetTypeUrl())
	return nil
}

func (c *Callbacks) OnStreamDeltaResponse(streamID int64, req *discoveryv3.DeltaDiscoveryRequest, resp *discoveryv3.DeltaDiscoveryResponse) {
	slog.Debug("xDS delta stream response", "stream_id", streamID, "type_url", resp.GetTypeUrl())
}

var _ server.Callbacks = (*Callbacks)(nil)
