package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	SyncCloseDialError = "dial_error"
	SyncCloseSendError = "send_error"
	SyncCloseRecvError = "recv_error"
	SyncCloseOK        = "ok"
	SyncMsgInitial     = "initial"
	SyncMsgUpdate      = "update"
	SyncMsgHeartbeat   = "heartbeat"
)

var (
	controllerStreamOpens = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "auth_sidecar_controller_sync_stream_opens_total",
			Help: "Successful SyncAccess stream handshakes (after initial Send).",
		},
	)
	controllerStreamCloses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_sidecar_controller_sync_stream_closes_total",
			Help: "Sync stream ends by reason.",
		},
		[]string{"reason"},
	)
	controllerReconnects = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "auth_sidecar_controller_sync_reconnects_total",
			Help: "Backoff reconnects after controller sync stream failure.",
		},
	)
	controllerSyncMessages = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_sidecar_controller_sync_messages_total",
			Help: "Messages received on SyncAccess stream.",
		},
		[]string{"kind"},
	)
	controllerConnected = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "auth_sidecar_controller_connected",
			Help: "1 if SyncAccess stream is active, else 0.",
		},
	)
	accessContractsCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "auth_sidecar_access_contracts_count",
			Help: "Number of contracts in in-memory access storage.",
		},
	)
)

func RecordControllerStreamOpen(enabled bool) {
	if !enabled {
		return
	}
	controllerStreamOpens.Inc()
}

func RecordControllerStreamClose(enabled bool, reason string) {
	if !enabled {
		return
	}
	controllerStreamCloses.WithLabelValues(reason).Inc()
}

func RecordControllerReconnect(enabled bool) {
	if !enabled {
		return
	}
	controllerReconnects.Inc()
}

func RecordControllerSyncMessage(enabled bool, kind string) {
	if !enabled {
		return
	}
	controllerSyncMessages.WithLabelValues(kind).Inc()
}

func SetControllerConnected(enabled bool, connected bool) {
	if !enabled {
		return
	}
	if connected {
		controllerConnected.Set(1)
	} else {
		controllerConnected.Set(0)
	}
}

// SetAccessContractsCount updates gauge of stored contracts (call after config apply).
func SetAccessContractsCount(enabled bool, n int) {
	if !enabled {
		return
	}
	accessContractsCount.Set(float64(n))
}
