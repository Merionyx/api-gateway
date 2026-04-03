package xds

import (
	"context"
	"log/slog"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
)

type Callbacks struct{}

func (c *Callbacks) OnStreamOpen(ctx context.Context, streamID int64, typeURL string) error {
	slog.Debug("xDS stream opened", "stream_id", streamID, "type_url", typeURL)
	return nil
}

func (c *Callbacks) OnStreamClosed(streamID int64, node *corev3.Node) {
	slog.Debug("xDS stream closed", "stream_id", streamID)
}

func (c *Callbacks) OnStreamRequest(streamID int64, req *discoveryv3.DiscoveryRequest) error {
	slog.Debug("xDS stream request",
		"stream_id", streamID, "node", req.GetNode().GetId(), "version", req.GetVersionInfo())
	return nil
}

func (c *Callbacks) OnStreamResponse(ctx context.Context, streamID int64, req *discoveryv3.DiscoveryRequest, resp *discoveryv3.DiscoveryResponse) {
	slog.Debug("xDS stream response",
		"stream_id", streamID, "type_url", resp.GetTypeUrl(), "version", resp.GetVersionInfo())
}

func (c *Callbacks) OnFetchRequest(ctx context.Context, req *discoveryv3.DiscoveryRequest) error {
	return nil
}

func (c *Callbacks) OnFetchResponse(req *discoveryv3.DiscoveryRequest, resp *discoveryv3.DiscoveryResponse) {
}

// Delta xDS methods
func (c *Callbacks) OnDeltaStreamOpen(ctx context.Context, streamID int64, typeURL string) error {
	slog.Debug("xDS delta stream opened", "stream_id", streamID, "type_url", typeURL)
	return nil
}
func (c *Callbacks) OnDeltaStreamClosed(streamID int64, node *corev3.Node) {
	slog.Debug("xDS delta stream closed", "stream_id", streamID)
}
func (c *Callbacks) OnStreamDeltaRequest(streamID int64, req *discoveryv3.DeltaDiscoveryRequest) error {
	slog.Debug("xDS delta stream request", "stream_id", streamID, "node", req.GetNode().GetId())
	return nil
}
func (c *Callbacks) OnStreamDeltaResponse(streamID int64, req *discoveryv3.DeltaDiscoveryRequest, resp *discoveryv3.DeltaDiscoveryResponse) {
	slog.Debug("xDS delta stream response", "stream_id", streamID, "type_url", resp.GetTypeUrl())
}

var _ server.Callbacks = (*Callbacks)(nil)
