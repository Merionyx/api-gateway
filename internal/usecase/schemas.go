package usecase

import (
	"context"
	"fmt"
	"log"

	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/domain/models"
	"merionyx/api-gateway/control-plane/internal/repository/git"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type schemasUseCase struct {
	schemaRepo      interfaces.SchemaRepository
	environmentRepo interfaces.EnvironmentRepository
	gitManager      *git.RepositoryManager
}

func NewSchemasUseCase() interfaces.SchemasUseCase {
	return &schemasUseCase{}
}

func (uc *schemasUseCase) SetDependencies(schemaRepo interfaces.SchemaRepository, environmentRepo interfaces.EnvironmentRepository, gitManager *git.RepositoryManager) {
	uc.schemaRepo = schemaRepo
	uc.environmentRepo = environmentRepo
	uc.gitManager = gitManager
}

func (uc *schemasUseCase) SyncContractBundle(ctx context.Context, req *models.SyncContractBundleRequest) (*models.SyncContractBundleResponse, error) {
	// Выкачиваем из Git
	log.Printf("Syncing contract %s/%s/%s from Git", req.Repository, req.Ref, req.Bundle)
	snapshots, err := uc.gitManager.GetRepositorySnapshots(req.Repository, req.Ref, req.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository snapshots: %w", err)
	}

	for _, snapshot := range snapshots {
		if err := uc.schemaRepo.SaveContractSnapshot(ctx, req.Repository, req.Ref, snapshot.Name, &snapshot); err != nil {
			return nil, fmt.Errorf("failed to save contract snapshot to etcd: %w", err)
		}

		log.Printf("Contract %s/%s/%s synced and saved to etcd", req.Repository, req.Ref, snapshot.Name)
	}

	return &models.SyncContractBundleResponse{
		Snapshots: snapshots,
		FromCache: false,
	}, nil
}

func (uc *schemasUseCase) GetContractSnapshot(ctx context.Context, repository, ref, contract string) (*git.ContractSnapshot, error) {
	return uc.schemaRepo.GetContractSnapshot(ctx, repository, ref, contract)
}

func (uc *schemasUseCase) ListContractSnapshots(ctx context.Context, repository, ref string) ([]git.ContractSnapshot, error) {
	return uc.schemaRepo.ListContractSnapshots(ctx, repository, ref)
}

func (uc *schemasUseCase) SyncAllContracts(ctx context.Context, req *models.SyncAllContractsRequest) (*models.SyncAllContractsResponse, error) {
	var environments map[string]*models.Environment
	var err error

	if req.Environment != "" {
		// Синхронизируем контракты для одного окружения
		env, err := uc.environmentRepo.GetEnvironment(ctx, req.Environment)
		if err != nil {
			return nil, fmt.Errorf("environment %s not found: %w", req.Environment, err)
		}
		environments = map[string]*models.Environment{req.Environment: env}
	} else {
		// Синхронизируем контракты для всех окружений
		environments, err = uc.environmentRepo.ListEnvironments(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list environments: %w", err)
		}
	}

	var syncedCount int32
	var errors []string

	for envName, env := range environments {
		log.Printf("Syncing contracts for environment: %s", envName)

		for _, bundle := range env.Bundles.Static {
			syncReq := &models.SyncContractBundleRequest{
				Repository: bundle.Repository,
				Ref:        bundle.Ref,
				Bundle:     bundle.Name,
				Path:       bundle.Path,
				Force:      true,
			}

			_, err := uc.SyncContractBundle(ctx, syncReq)
			if err != nil {
				errMsg := fmt.Sprintf("failed to sync contract %s/%s/%s: %v",
					bundle.Repository, bundle.Ref, bundle.Name, err)
				log.Printf("Error: %s", errMsg)
				errors = append(errors, errMsg)
			} else {
				syncedCount++
			}
		}
	}

	log.Printf("Sync completed: %d contracts synced, %d errors", syncedCount, len(errors))

	return &models.SyncAllContractsResponse{
		SyncedCount: syncedCount,
		Errors:      errors,
	}, nil
}

func (uc *schemasUseCase) WatchContractBundlesSnapshots(ctx context.Context) clientv3.WatchChan {
	return uc.schemaRepo.WatchContractBundlesSnapshots(ctx)
}
