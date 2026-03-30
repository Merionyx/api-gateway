package handler

import (
	"context"

	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/domain/models"
	snapshotsv1 "merionyx/api-gateway/control-plane/pkg/api/snapshots/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SnapshotHandler struct {
	snapshotsv1.UnimplementedSnapshotsServiceServer
	snapshotsUseCase interfaces.SnapshotsUseCase
}

func NewSnapshotHandler(snapshotsUseCase interfaces.SnapshotsUseCase) *SnapshotHandler {
	return &SnapshotHandler{snapshotsUseCase: snapshotsUseCase}
}

func (h *SnapshotHandler) UpdateSnapshot(ctx context.Context, req *snapshotsv1.UpdateSnapshotRequest) (*snapshotsv1.UpdateSnapshotResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be empty")
	}

	response, err := h.snapshotsUseCase.UpdateSnapshot(ctx, &models.UpdateSnapshotRequest{
		Environment: req.Environment,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &snapshotsv1.UpdateSnapshotResponse{
		Success:             response.Success,
		UpdatedEnvironments: response.UpdatedEnvironments,
	}, nil
}

func (h *SnapshotHandler) GetSnapshotStatus(ctx context.Context, req *snapshotsv1.GetSnapshotStatusRequest) (*snapshotsv1.GetSnapshotStatusResponse, error) {
	if req == nil || req.Environment == "" {
		return nil, status.Error(codes.InvalidArgument, "environment is required")
	}

	response, err := h.snapshotsUseCase.GetSnapshotStatus(ctx, &models.GetSnapshotStatusRequest{
		Environment: req.Environment,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &snapshotsv1.GetSnapshotStatusResponse{
		Environment:    response.Environment,
		Version:        response.Version,
		ContractsCount: response.ContractsCount,
		ClustersCount:  response.ClustersCount,
		RoutesCount:    response.RoutesCount,
	}, nil
}
