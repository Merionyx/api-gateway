package handler

import (
	"context"
	"log/slog"

	"merionyx/api-gateway/internal/contract-syncer/domain/interfaces"
	contractv1 "merionyx/api-gateway/pkg/api/contract/v1"
	pb "merionyx/api-gateway/pkg/api/contract_syncer/v1"
)

type SyncHandler struct {
	pb.UnimplementedContractSyncerServiceServer
	syncUseCase interfaces.SyncUseCase
}

func NewSyncHandler(syncUseCase interfaces.SyncUseCase) *SyncHandler {
	return &SyncHandler{
		syncUseCase: syncUseCase,
	}
}

// Sync is stateless: safe behind TCP load balancing; callers (API Server leader) retry on failure.
func (h *SyncHandler) Sync(ctx context.Context, req *pb.SyncRequest) (*pb.SyncResponse, error) {
	slog.Info("Received sync request", "repository", req.Repository, "ref", req.Ref, "path", req.Path)

	snapshots, err := h.syncUseCase.Sync(req.Repository, req.Ref, req.Path)
	if err != nil {
		slog.Error("Failed to sync repository", "error", err)
		return &pb.SyncResponse{
			Error: err.Error(),
		}, nil
	}

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
