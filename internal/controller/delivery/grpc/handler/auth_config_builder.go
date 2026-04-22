package handler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	authv1 "github.com/merionyx/api-gateway/pkg/grpc/auth/v1"

	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/shared/utils"
)

// AuthConfigBuilder builds authv1.AccessConfig for sidecar: controller etcd, then in-memory, then slice
// from schema repo (env patterns). See auth_config_builder_test.go.
type AuthConfigBuilder struct {
	environmentRepo interfaces.EnvironmentRepository
	inMemory        interfaces.InMemoryEnvironmentsRepository
	schemaRepo      interfaces.SchemaRepository
}

// NewAuthConfigBuilder — dependencies for building; without gRPC, subscriptions and watch.
func NewAuthConfigBuilder(
	environmentRepo interfaces.EnvironmentRepository,
	inMemory interfaces.InMemoryEnvironmentsRepository,
	schema interfaces.SchemaRepository,
) *AuthConfigBuilder {
	return &AuthConfigBuilder{
		environmentRepo: environmentRepo,
		inMemory:        inMemory,
		schemaRepo:      schema,
	}
}

// BuildAccessConfig — logic of former AuthHandler.buildAccessConfig (moved to p.7).
func (b *AuthConfigBuilder) BuildAccessConfig(ctx context.Context, environment string) (*authv1.AccessConfig, error) {
	var env *models.Environment

	config := &authv1.AccessConfig{
		Environment: environment,
		Contracts:   make([]*authv1.ContractAccess, 0),
		Version:     time.Now().Unix(),
	}

	env, err := b.environmentRepo.GetEnvironment(ctx, environment)
	if err != nil {
		env, err = b.inMemory.GetEnvironment(ctx, environment)
		if err != nil {
			return config, fmt.Errorf("environment not found: %w", err)
		}
		for _, snapshot := range env.Snapshots {
			appendContractAccessForEnvironment(config, environment, snapshot, appMatchSnapshotFromMemory)
		}
	}

	if env != nil && env.Bundles != nil {
		for _, bundle := range env.Bundles.Static {
			snapshots, err := b.schemaRepo.ListContractSnapshots(ctx, bundle.Repository, bundle.Ref, bundle.Path)
			if err != nil {
				slog.Warn("auth sync: failed to list snapshots for bundle", "bundle", bundle.Name, "error", err)
				continue
			}
			for _, snapshot := range snapshots {
				appendContractAccessForEnvironment(config, environment, snapshot, appMatchSchemaBundle)
			}
		}
	}

	return config, nil
}

// appMatchMode — matching app.Environments with environment name (in-memory slice vs schema).
type appMatchMode int

// Semantics: in-memory slice — exact equality of [models.App] env cells; schema — [utils.MatchesEnvironmentPattern].
const (
	appMatchSnapshotFromMemory appMatchMode = iota
	appMatchSchemaBundle
)

func appAllowedForEnvironment(environment string, app models.App, mode appMatchMode) bool {
	if len(app.Environments) == 0 {
		return true
	}
	for _, e := range app.Environments {
		switch mode {
		case appMatchSnapshotFromMemory:
			if e == environment {
				return true
			}
		case appMatchSchemaBundle:
			if utils.MatchesEnvironmentPattern(environment, e) {
				return true
			}
		}
	}
	return false
}

func appendContractAccessForEnvironment(
	config *authv1.AccessConfig,
	environment string,
	snapshot models.ContractSnapshot,
	mode appMatchMode,
) {
	contractAccess := &authv1.ContractAccess{
		ContractName: snapshot.Name,
		Prefix:       snapshot.Prefix,
		Secure:       snapshot.Access.Secure,
		Apps:         make([]*authv1.AppAccess, 0),
	}
	for _, app := range snapshot.Access.Apps {
		if !appAllowedForEnvironment(environment, app, mode) {
			continue
		}
		contractAccess.Apps = append(contractAccess.Apps, &authv1.AppAccess{
			AppId:        app.AppID,
			Environments: app.Environments,
		})
	}
	config.Contracts = append(config.Contracts, contractAccess)
}
