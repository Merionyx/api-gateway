package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

type environmentsUseCase struct {
	environmentRepo interfaces.EnvironmentRepository
	inMemoryEnvs    interfaces.InMemoryEnvironmentsRepository
	schemasUseCase  interfaces.SchemasUseCase
	eff             interfaces.EffectiveReconciler
}

func NewEnvironmentsUseCase() interfaces.EnvironmentsUseCase {
	return &environmentsUseCase{}
}

func (uc *environmentsUseCase) SetDependencies(
	environmentRepo interfaces.EnvironmentRepository,
	inMemory interfaces.InMemoryEnvironmentsRepository,
	schemasUseCase interfaces.SchemasUseCase,
	eff interfaces.EffectiveReconciler,
) {
	uc.environmentRepo = environmentRepo
	uc.inMemoryEnvs = inMemory
	uc.schemasUseCase = schemasUseCase
	uc.eff = eff
}

func (uc *environmentsUseCase) CreateEnvironment(ctx context.Context, req *models.CreateEnvironmentRequest) (*models.Environment, error) {
	// Check if environment does not exist
	existing, _ := uc.environmentRepo.GetEnvironment(ctx, req.Name)
	if existing != nil {
		return nil, fmt.Errorf("environment %s already exists", req.Name)
	}

	env := &models.Environment{
		Name:      req.Name,
		Type:      req.Type,
		Bundles:   req.Bundles,
		Services:  req.Services,
		Snapshots: make([]models.ContractSnapshot, 0),
	}

	// Save to etcd
	if err := uc.environmentRepo.SaveEnvironment(ctx, env); err != nil {
		return nil, fmt.Errorf("failed to save environment: %w", err)
	}

	slog.Info("environment created", "name", req.Name)

	if uc.eff != nil {
		if err := uc.eff.ReconcileOne(ctx, req.Name, true); err != nil {
			slog.Warn("reconcile xDS / materialized after create", "environment", req.Name, "error", err)
		}
	}

	return env, nil
}

func (uc *environmentsUseCase) GetEnvironment(ctx context.Context, name string) (*models.Environment, error) {
	env, err := uc.environmentRepo.GetEnvironment(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}

	for _, bundle := range env.Bundles.Static {
		snapshots, err := uc.schemasUseCase.ListContractSnapshots(ctx, bundle.Repository, bundle.Ref, bundle.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to get environment snapshots: %w", err)
		}
		env.Snapshots = append(env.Snapshots, snapshots...)
	}

	return env, nil
}

func (uc *environmentsUseCase) ListEnvironments(ctx context.Context) (map[string]*models.Environment, error) {
	environments, err := uc.environmentRepo.ListEnvironments(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}
	for _, environment := range environments {
		for _, bundle := range environment.Bundles.Static {
			snapshots, err := uc.schemasUseCase.ListContractSnapshots(ctx, bundle.Repository, bundle.Ref, bundle.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to get environment snapshots: %w", err)
			}
			environment.Snapshots = append(environment.Snapshots, snapshots...)
		}
		environments[environment.Name] = environment
	}
	return environments, nil
}

func (uc *environmentsUseCase) UpdateEnvironment(ctx context.Context, req *models.UpdateEnvironmentRequest) (*models.Environment, error) {
	// Check if environment exists
	existing, err := uc.environmentRepo.GetEnvironment(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("environment %s not found: %w", req.Name, err)
	}

	// Update fields
	existing.Bundles = req.Bundles
	existing.Services = req.Services

	// Save to etcd
	if err := uc.environmentRepo.SaveEnvironment(ctx, existing); err != nil {
		return nil, fmt.Errorf("failed to update environment: %w", err)
	}

	slog.Info("environment updated", "name", req.Name)

	if uc.eff != nil {
		if err := uc.eff.ReconcileOne(ctx, req.Name, true); err != nil {
			slog.Warn("reconcile xDS / materialized after update", "environment", req.Name, "error", err)
		}
	}

	return existing, nil
}

func (uc *environmentsUseCase) DeleteEnvironment(ctx context.Context, name string) error {
	// Check if environment exists
	_, err := uc.environmentRepo.GetEnvironment(ctx, name)
	if err != nil {
		return fmt.Errorf("environment %s not found: %w", name, err)
	}

	// Delete from etcd
	if err := uc.environmentRepo.DeleteEnvironment(ctx, name); err != nil {
		return fmt.Errorf("failed to delete environment: %w", err)
	}

	slog.Info("environment deleted", "name", name)

	if uc.eff != nil {
		if err := uc.eff.ReconcileOne(ctx, name, true); err != nil {
			return fmt.Errorf("reconcile after delete: %w", err)
		}
	}

	return nil
}
