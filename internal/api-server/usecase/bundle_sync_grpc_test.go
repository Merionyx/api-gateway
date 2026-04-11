package usecase

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	commonv1 "github.com/merionyx/api-gateway/pkg/grpc/common/v1"
	pb "github.com/merionyx/api-gateway/pkg/grpc/contract_syncer/v1"

	contractsyncergrpc "github.com/merionyx/api-gateway/internal/api-server/adapter/contractsyncer/grpc"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
	"github.com/merionyx/api-gateway/internal/shared/grpcobs"

	"google.golang.org/grpc"
)

type fakeSnapshotRepo struct {
	lastKey   string
	lastSnaps []sharedgit.ContractSnapshot
}

func (f *fakeSnapshotRepo) SaveSnapshots(_ context.Context, bundleKey string, snapshots []sharedgit.ContractSnapshot) (bool, error) {
	f.lastKey = bundleKey
	f.lastSnaps = append([]sharedgit.ContractSnapshot(nil), snapshots...)
	return true, nil
}

func (f *fakeSnapshotRepo) GetSnapshots(context.Context, string) ([]sharedgit.ContractSnapshot, error) {
	return nil, nil
}

func (f *fakeSnapshotRepo) ListBundleKeys(context.Context) ([]string, error) { return nil, nil }

type grpcContractSyncerMock struct {
	pb.UnimplementedContractSyncerServiceServer
	errMsg string
	snaps  []*commonv1.ContractSnapshot
}

func (m *grpcContractSyncerMock) Sync(context.Context, *pb.SyncRequest) (*pb.SyncResponse, error) {
	if m.errMsg != "" {
		return &pb.SyncResponse{Error: m.errMsg}, nil
	}
	return &pb.SyncResponse{Snapshots: m.snaps}, nil
}

func startContractSyncerGRPC(t *testing.T, impl pb.ContractSyncerServiceServer) (addr string, stop func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := grpc.NewServer()
	pb.RegisterContractSyncerServiceServer(s, impl)
	go func() { _ = s.Serve(lis) }()
	return lis.Addr().String(), func() {
		s.Stop()
		_ = lis.Close()
	}
}

func TestBundleSyncUseCase_SyncBundle_ContextCanceled(t *testing.T) {
	repo := &fakeSnapshotRepo{}
	client := contractsyncergrpc.NewClient("127.0.0.1:1", grpcobs.ClientTLSConfig{})
	uc := NewBundleSyncUseCase(repo, nil, client, nil, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := uc.SyncBundle(ctx, models.BundleInfo{Repository: "r", Ref: "main", Path: "p"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want Canceled, got %v", err)
	}
}

func TestBundleSyncUseCase_SyncBundle_Success(t *testing.T) {
	mock := &grpcContractSyncerMock{
		snaps: []*commonv1.ContractSnapshot{
			{
				Name:   "c1",
				Prefix: "/api/",
				Upstream: &commonv1.ContractUpstream{
					Name: "upstream-svc",
				},
				Access: &commonv1.Access{
					Secure: true,
					Apps: []*commonv1.App{
						{AppId: "a1", Environments: []string{"dev"}},
					},
				},
			},
		},
	}
	addr, stop := startContractSyncerGRPC(t, mock)
	defer stop()

	repo := &fakeSnapshotRepo{}
	client := contractsyncergrpc.NewClient(addr, grpcobs.ClientTLSConfig{})
	uc := NewBundleSyncUseCase(repo, nil, client, nil, false)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	snaps, err := uc.SyncBundle(ctx, models.BundleInfo{Repository: "repo", Ref: "v1", Path: "openapi"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].Name != "c1" {
		t.Fatalf("snaps %+v", snaps)
	}
	if len(repo.lastSnaps) != 1 || repo.lastKey == "" {
		t.Fatalf("repo lastKey=%q lastSnaps=%d", repo.lastKey, len(repo.lastSnaps))
	}
}

func TestBundleSyncUseCase_SyncBundle_RejectedResponse(t *testing.T) {
	mock := &grpcContractSyncerMock{errMsg: "invalid bundle"}
	addr, stop := startContractSyncerGRPC(t, mock)
	defer stop()

	client := contractsyncergrpc.NewClient(addr, grpcobs.ClientTLSConfig{})
	uc := NewBundleSyncUseCase(&fakeSnapshotRepo{}, nil, client, nil, false)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := uc.SyncBundle(ctx, models.BundleInfo{Repository: "r", Ref: "x", Path: "p"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, apierrors.ErrContractSyncerRejected) {
		t.Fatalf("want ErrContractSyncerRejected, got %v", err)
	}
}
