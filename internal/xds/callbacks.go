package xds

import (
	"context"
	"log"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
)

type Callbacks struct{}

func (c *Callbacks) OnStreamOpen(ctx context.Context, streamID int64, typeURL string) error {
	log.Printf("Stream opened: streamID=%d, typeURL=%s", streamID, typeURL)
	return nil
}

func (c *Callbacks) OnStreamClosed(streamID int64, node *corev3.Node) {
	log.Printf("Stream closed: streamID=%d", streamID)
}

func (c *Callbacks) OnStreamRequest(streamID int64, req *discoveryv3.DiscoveryRequest) error {
	log.Printf("Stream request: streamID=%d, node=%s, version=%s",
		streamID, req.GetNode().GetId(), req.GetVersionInfo())
	return nil
}

func (c *Callbacks) OnStreamResponse(ctx context.Context, streamID int64, req *discoveryv3.DiscoveryRequest, resp *discoveryv3.DiscoveryResponse) {
	log.Printf("Stream response: streamID=%d, typeURL=%s, version=%s",
		streamID, resp.GetTypeUrl(), resp.GetVersionInfo())
}

func (c *Callbacks) OnFetchRequest(ctx context.Context, req *discoveryv3.DiscoveryRequest) error {
	return nil
}

func (c *Callbacks) OnFetchResponse(req *discoveryv3.DiscoveryRequest, resp *discoveryv3.DiscoveryResponse) {
}

// Delta xDS methods
func (c *Callbacks) OnDeltaStreamOpen(ctx context.Context, streamID int64, typeURL string) error {
	log.Printf("Delta stream opened: streamID=%d, typeURL=%s", streamID, typeURL)
	return nil
}
func (c *Callbacks) OnDeltaStreamClosed(streamID int64, node *corev3.Node) {
	log.Printf("Delta stream closed: streamID=%d", streamID)
}
func (c *Callbacks) OnStreamDeltaRequest(streamID int64, req *discoveryv3.DeltaDiscoveryRequest) error {
	log.Printf("Delta stream request: streamID=%d, node=%s",
		streamID, req.GetNode().GetId())
	return nil
}
func (c *Callbacks) OnStreamDeltaResponse(streamID int64, req *discoveryv3.DeltaDiscoveryRequest, resp *discoveryv3.DeltaDiscoveryResponse) {
	log.Printf("Delta stream response: streamID=%d, typeURL=%s",
		streamID, resp.GetTypeUrl())
}

// Проверяем, что Callbacks реализует интерфейс
var _ server.Callbacks = (*Callbacks)(nil)
