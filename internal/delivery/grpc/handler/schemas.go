package handler

import (
	"context"

	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/domain/models"
	"merionyx/api-gateway/control-plane/internal/repository/git"
	schemasv1 "merionyx/api-gateway/control-plane/pkg/api/schemas/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SchemasHandler struct {
	schemasv1.UnimplementedSchemasServiceServer
	schemasUseCase interfaces.SchemasUseCase
}

func NewSchemasHandler(schemasUseCase interfaces.SchemasUseCase) *SchemasHandler {
	return &SchemasHandler{schemasUseCase: schemasUseCase}
}

func (h *SchemasHandler) SyncContractBundle(ctx context.Context, req *schemasv1.SyncContractBundleRequest) (*schemasv1.SyncContractBundleResponse, error) {
	if req == nil || req.Repository == "" || req.Ref == "" || req.Bundle == "" {
		return nil, status.Error(codes.InvalidArgument, "repository, ref, and bundle are required")
	}

	response, err := h.schemasUseCase.SyncContractBundle(ctx, &models.SyncContractBundleRequest{
		Repository: req.Repository,
		Ref:        req.Ref,
		Bundle:     req.Bundle,
		Force:      req.Force,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &schemasv1.SyncContractBundleResponse{
		Snapshots: modelToProtoContractSnapshots(response.Snapshots),
		FromCache: response.FromCache,
	}, nil
}

func (h *SchemasHandler) GetContractSnapshot(ctx context.Context, req *schemasv1.GetContractSnapshotRequest) (*schemasv1.GetContractSnapshotResponse, error) {
	if req == nil || req.Repository == "" || req.Ref == "" || req.Contract == "" {
		return nil, status.Error(codes.InvalidArgument, "repository, ref, and contract are required")
	}

	snapshot, err := h.schemasUseCase.GetContractSnapshot(ctx, req.Repository, req.Ref, req.Contract)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return &schemasv1.GetContractSnapshotResponse{
		Snapshot: modelToProtoContractSnapshot(snapshot),
	}, nil
}

func (h *SchemasHandler) ListContractSnapshots(ctx context.Context, req *schemasv1.ListContractSnapshotsRequest) (*schemasv1.ListContractSnapshotsResponse, error) {
	if req == nil || req.Repository == "" || req.Ref == "" {
		return nil, status.Error(codes.InvalidArgument, "repository and ref are required")
	}

	snapshots, err := h.schemasUseCase.ListContractSnapshots(ctx, req.Repository, req.Ref)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var protoSnapshots []*schemasv1.ContractSnapshot
	for _, s := range snapshots {
		protoSnapshots = append(protoSnapshots, modelToProtoContractSnapshot(&s))
	}

	return &schemasv1.ListContractSnapshotsResponse{
		Snapshots: protoSnapshots,
	}, nil
}

func (h *SchemasHandler) SyncAllContracts(ctx context.Context, req *schemasv1.SyncAllContractsRequest) (*schemasv1.SyncAllContractsResponse, error) {
	response, err := h.schemasUseCase.SyncAllContracts(ctx, &models.SyncAllContractsRequest{
		Environment: req.Environment,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &schemasv1.SyncAllContractsResponse{
		SyncedCount: response.SyncedCount,
		Errors:      response.Errors,
	}, nil
}

func modelToProtoContractSnapshot(snapshot *git.ContractSnapshot) *schemasv1.ContractSnapshot {
	if snapshot == nil {
		return nil
	}

	return &schemasv1.ContractSnapshot{
		Name:                  snapshot.Name,
		Prefix:                snapshot.Prefix,
		Upstream:              &schemasv1.ContractUpstream{Name: snapshot.Upstream.Name},
		AllowUndefinedMethods: snapshot.AllowUndefinedMethods,
	}
}

func modelToProtoContractSnapshots(snapshots []git.ContractSnapshot) []*schemasv1.ContractSnapshot {
	if snapshots == nil {
		return nil
	}

	var protoSnapshots []*schemasv1.ContractSnapshot
	for _, s := range snapshots {
		protoSnapshots = append(protoSnapshots, modelToProtoContractSnapshot(&s))
	}
	return protoSnapshots
}
