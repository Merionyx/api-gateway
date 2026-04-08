package usecase

import (
	"context"
	"fmt"

	"merionyx/api-gateway/internal/shared/grpcobs"
	sharedgit "merionyx/api-gateway/internal/shared/git"
	pb "merionyx/api-gateway/pkg/api/contract_syncer/v1"

	"google.golang.org/grpc"
)

// ContractExportUseCase proxies contract file export to Contract Syncer (no etcd).
type ContractExportUseCase struct {
	addr string
	tls  grpcobs.ClientTLSConfig
}

func NewContractExportUseCase(addr string, tls grpcobs.ClientTLSConfig) *ContractExportUseCase {
	return &ContractExportUseCase{addr: addr, tls: tls}
}

func (u *ContractExportUseCase) Export(ctx context.Context, repository, ref, path, contractName string) ([]sharedgit.ExportedContractFile, error) {
	opts, err := ContractSyncerDialOptions(u.tls)
	if err != nil {
		return nil, fmt.Errorf("contract syncer dial options: %w", err)
	}
	conn, err := grpc.NewClient(u.addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("dial contract syncer: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	client := pb.NewContractSyncerServiceClient(conn)
	resp, err := client.ExportContracts(ctx, &pb.ExportContractsRequest{
		Repository:   repository,
		Ref:          ref,
		Path:         path,
		ContractName: contractName,
	})
	if err != nil {
		return nil, fmt.Errorf("export contracts rpc: %w", err)
	}
	if resp.GetError() != "" {
		return nil, fmt.Errorf("%w: %s", ErrContractSyncerRejected, resp.GetError())
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
