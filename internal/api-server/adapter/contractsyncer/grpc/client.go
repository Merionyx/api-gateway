package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	commonv1 "github.com/merionyx/api-gateway/pkg/grpc/common/v1"
	pb "github.com/merionyx/api-gateway/pkg/grpc/contract_syncer/v1"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
	"github.com/merionyx/api-gateway/internal/shared/grpcobs"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// Client is the gRPC adapter for Contract Syncer (implements domain ports).
type Client struct {
	addr string
	tls  grpcobs.ClientTLSConfig
}

// NewClient returns a Contract Syncer gRPC client. The same instance may satisfy
// interfaces.ContractSyncRemote, ContractExportRemote, and ContractSyncerReachability.
func NewClient(addr string, tls grpcobs.ClientTLSConfig) *Client {
	return &Client{addr: addr, tls: tls}
}

// FetchContractSnapshots implements interfaces.ContractSyncRemote.
func (c *Client) FetchContractSnapshots(ctx context.Context, bundle models.BundleInfo) ([]sharedgit.ContractSnapshot, error) {
	dialOpts, err := DialOptions(c.tls)
	if err != nil {
		return nil, apierrors.JoinContractSyncer("contract syncer dial options", err)
	}
	conn, err := grpc.NewClient(c.addr, dialOpts...)
	if err != nil {
		return nil, apierrors.JoinContractSyncer("dial contract syncer", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			slog.Debug("contract syncer grpc: close conn", "error", cerr)
		}
	}()

	client := pb.NewContractSyncerServiceClient(conn)
	resp, err := client.Sync(ctx, &pb.SyncRequest{
		Repository: bundle.Repository,
		Ref:        bundle.Ref,
		Path:       bundle.Path,
	})
	if err != nil {
		return nil, apierrors.JoinContractSyncer("sync rpc", err)
	}
	if resp.GetError() != "" {
		return nil, fmt.Errorf("%w: %s", apierrors.ErrContractSyncerRejected, resp.GetError())
	}

	return mapCommonSnapshotsToDomain(resp.GetSnapshots()), nil
}

// ExportContractFiles implements interfaces.ContractExportRemote.
func (c *Client) ExportContractFiles(ctx context.Context, repository, ref, path, contractName string) ([]sharedgit.ExportedContractFile, error) {
	opts, err := DialOptions(c.tls)
	if err != nil {
		return nil, apierrors.JoinContractSyncer("contract syncer dial options", err)
	}
	conn, err := grpc.NewClient(c.addr, opts...)
	if err != nil {
		return nil, apierrors.JoinContractSyncer("dial contract syncer", err)
	}
	defer func() { _ = conn.Close() }()

	client := pb.NewContractSyncerServiceClient(conn)
	resp, err := client.ExportContracts(ctx, &pb.ExportContractsRequest{
		Repository:   repository,
		Ref:          ref,
		Path:         path,
		ContractName: contractName,
	})
	if err != nil {
		return nil, apierrors.JoinContractSyncer("export contracts rpc", err)
	}
	if resp.GetError() != "" {
		return nil, fmt.Errorf("%w: %s", apierrors.ErrContractSyncerRejected, resp.GetError())
	}

	out := make([]sharedgit.ExportedContractFile, 0, len(resp.Files))
	for _, f := range resp.Files {
		out = append(out, sharedgit.ExportedContractFile{
			ContractName: f.GetContractName(),
			SourcePath:   f.GetSourcePath(),
			Content:      f.GetContent(),
		})
	}
	return out, nil
}

// Ping implements interfaces.ContractSyncerReachability.
func (c *Client) Ping(ctx context.Context) error {
	if c.addr == "" {
		return fmt.Errorf("%w: contract syncer address not configured", apierrors.ErrInvalidInput)
	}
	opts, err := DialOptions(c.tls)
	if err != nil {
		return apierrors.JoinContractSyncer("contract syncer dial options", err)
	}
	conn, err := grpc.NewClient(c.addr, opts...)
	if err != nil {
		return apierrors.JoinContractSyncer("dial contract syncer", err)
	}
	defer func() { _ = conn.Close() }()
	conn.Connect()
	for {
		st := conn.GetState()
		if st == connectivity.Ready {
			return nil
		}
		if st == connectivity.Shutdown {
			return apierrors.JoinContractSyncer("wait for ready", errors.New("connection shutdown"))
		}
		if !conn.WaitForStateChange(ctx, st) {
			return ctx.Err()
		}
	}
}

func mapCommonSnapshotsToDomain(pbSnaps []*commonv1.ContractSnapshot) []sharedgit.ContractSnapshot {
	var snapshots []sharedgit.ContractSnapshot
	for _, pbSnapshot := range pbSnaps {
		if pbSnapshot == nil {
			continue
		}
		var apps []sharedgit.App
		if acc := pbSnapshot.GetAccess(); acc != nil {
			for _, pbApp := range acc.GetApps() {
				apps = append(apps, sharedgit.App{
					AppID:        pbApp.GetAppId(),
					Environments: pbApp.GetEnvironments(),
				})
			}
		}
		upstreamName := ""
		if u := pbSnapshot.GetUpstream(); u != nil {
			upstreamName = u.GetName()
		}
		secure := false
		if acc := pbSnapshot.GetAccess(); acc != nil {
			secure = acc.GetSecure()
		}
		snapshots = append(snapshots, sharedgit.ContractSnapshot{
			Name:                  pbSnapshot.GetName(),
			Prefix:                pbSnapshot.GetPrefix(),
			Upstream:              sharedgit.ContractUpstream{Name: upstreamName},
			AllowUndefinedMethods: pbSnapshot.GetAllowUndefinedMethods(),
			Access: sharedgit.Access{
				Secure: secure,
				Apps:   apps,
			},
		})
	}
	return snapshots
}
