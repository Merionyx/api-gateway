package handler

import (
	"context"
	"log/slog"
	"time"

	"merionyx/api-gateway/internal/contract-syncer/domain/interfaces"
	syncmetrics "merionyx/api-gateway/internal/contract-syncer/metrics"
	contractv1 "merionyx/api-gateway/pkg/api/contract/v1"
	pb "merionyx/api-gateway/pkg/api/contract_syncer/v1"
)

type SyncHandler struct {
	pb.UnimplementedContractSyncerServiceServer
	syncUseCase    interfaces.SyncUseCase
	metricsEnabled bool
}

func NewSyncHandler(syncUseCase interfaces.SyncUseCase, metricsEnabled bool) *SyncHandler {
	return &SyncHandler{
		syncUseCase:    syncUseCase,
		metricsEnabled: metricsEnabled,
	}
}

// Sync is stateless: safe behind TCP load balancing; callers (API Server leader) retry on failure.
func (h *SyncHandler) Sync(ctx context.Context, req *pb.SyncRequest) (*pb.SyncResponse, error) {
	slog.Info("Received sync request", "repository", req.Repository, "ref", req.Ref, "path", req.Path)

	start := time.Now()
	snapshots, err := h.syncUseCase.Sync(req.Repository, req.Ref, req.Path)
	if err != nil {
		slog.Error("Failed to sync repository", "error", err)
		syncmetrics.RecordSync(h.metricsEnabled, syncmetrics.OutcomeResponseError, time.Since(start))
		return &pb.SyncResponse{
			Error: err.Error(),
		}, nil
	}
	syncmetrics.RecordSync(h.metricsEnabled, syncmetrics.OutcomeOK, time.Since(start))

	var pbSnapshots []*contractv1.ContractSnapshot
	for _, snapshot := range snapshots {
		var pbApps []*contractv1.App
		for _, app := range snapshot.Access.Apps {
			pbApps = append(pbApps, &contractv1.App{
				AppId:        app.AppID,
				Environments: app.Environments,
			})
		}

		pbSnapshots = append(pbSnapshots, &contractv1.ContractSnapshot{
			Name:   snapshot.Name,
			Prefix: snapshot.Prefix,
			Upstream: &contractv1.ContractUpstream{
				Name: snapshot.Upstream.Name,
			},
			AllowUndefinedMethods: snapshot.AllowUndefinedMethods,
			Access: &contractv1.Access{
				Secure: snapshot.Access.Secure,
				Apps:   pbApps,
			},
		})
	}

	return &pb.SyncResponse{
		Snapshots: pbSnapshots,
	}, nil
}

func (h *SyncHandler) ExportContracts(ctx context.Context, req *pb.ExportContractsRequest) (*pb.ExportContractsResponse, error) {
	slog.Info("Received export request", "repository", req.Repository, "ref", req.Ref, "path", req.Path, "contract", req.ContractName)

	start := time.Now()
	files, err := h.syncUseCase.ExportContracts(req.Repository, req.Ref, req.Path, req.ContractName)
	if err != nil {
		slog.Error("Export contracts failed", "error", err)
		syncmetrics.RecordSync(h.metricsEnabled, syncmetrics.OutcomeResponseError, time.Since(start))
		return &pb.ExportContractsResponse{Error: err.Error()}, nil
	}
	syncmetrics.RecordSync(h.metricsEnabled, syncmetrics.OutcomeOK, time.Since(start))

	var out []*pb.ExportContractFile
	for i := range files {
		f := files[i]
		out = append(out, &pb.ExportContractFile{
			ContractName: f.ContractName,
			SourcePath:   f.SourcePath,
			Content:      f.Content,
		})
	}
	return &pb.ExportContractsResponse{Files: out}, nil
}
