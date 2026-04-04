package metrics

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	xdsStreamsOpen = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "gateway_controller_xds_streams_open",
			Help: "Number of open xDS ADS/SotW and delta streams.",
		},
	)
	xdsStreamRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_controller_xds_stream_requests_total",
			Help: "Discovery requests on xDS streams (resource type normalized).",
		},
		[]string{"type_url"},
	)
)

// NormalizeXDSResourceType maps full protobuf type URL to a short stable label.
func NormalizeXDSResourceType(typeURL string) string {
	if typeURL == "" {
		return "unknown"
	}
	if i := strings.LastIndex(typeURL, "/"); i >= 0 {
		return typeURL[i+1:]
	}
	return typeURL
}

func XDSStreamOpened(enabled bool) {
	if !enabled {
		return
	}
	xdsStreamsOpen.Inc()
}

func XDSStreamClosed(enabled bool) {
	if !enabled {
		return
	}
	xdsStreamsOpen.Dec()
}

func RecordXDSStreamRequest(enabled bool, typeURL string) {
	if !enabled {
		return
	}
	xdsStreamRequests.WithLabelValues(NormalizeXDSResourceType(typeURL)).Inc()
}
