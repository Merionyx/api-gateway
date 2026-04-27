package container

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	contractsyncergrpc "github.com/merionyx/api-gateway/internal/api-server/adapter/contractsyncer/grpc"
	"github.com/merionyx/api-gateway/internal/api-server/adapter/etcd"
	"github.com/merionyx/api-gateway/internal/api-server/auth/idpcache"
	authzroles "github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/auth/sessioncrypto"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	grpchandler "github.com/merionyx/api-gateway/internal/api-server/delivery/grpc/handler"
	httpauthz "github.com/merionyx/api-gateway/internal/api-server/delivery/http/authz"
	httphandler "github.com/merionyx/api-gateway/internal/api-server/delivery/http/handler"
	"github.com/merionyx/api-gateway/internal/api-server/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/api-server/idempotency"
	"github.com/merionyx/api-gateway/internal/api-server/metrics"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/bundle"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/registry"
	"github.com/merionyx/api-gateway/internal/shared/bootstrap"
	"github.com/merionyx/api-gateway/internal/shared/election"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Container DI for all dependencies
type Container struct {
	Config *config.Config

	EtcdClient *clientv3.Client
	LeaderGate election.LeaderGate

	SnapshotRepository    interfaces.SnapshotRepository
	ControllerRepository  interfaces.ControllerRepository
	APIKeyRepository      *etcd.APIKeyRepository
	SessionRepository     *etcd.SessionRepository
	LoginIntentRepository *etcd.LoginIntentRepository
	RoleCatalog           *authzroles.Catalog

	// ContractSyncerGRPC is the gRPC adapter for Contract Syncer (sync, export, ping).
	ContractSyncerGRPC *contractsyncergrpc.Client

	JWTUseCase          *auth.JWTUseCase
	OIDCLoginUseCase    *auth.OIDCLoginUseCase
	OIDCCallbackUseCase *auth.OIDCCallbackUseCase
	OIDCRefreshUseCase  *auth.OIDCRefreshUseCase
	OAuthTokenUseCase   *auth.OAuthTokenUseCase

	SessionSealer             *sessioncrypto.Keyring
	IdpAccessCache            *idpcache.Cache
	ControllerRegistryUseCase interfaces.ControllerRegistryUseCase
	BundleSyncUseCase         interfaces.BundleSyncUseCase
	PermissionEvaluator       *httpauthz.PermissionEvaluator

	JWTHandler *httphandler.JWTHandler

	OIDCLoginHandler *httphandler.OIDCLoginHandler

	OIDCCallbackHandler *httphandler.OIDCCallbackHandler
	OAuthTokenHandler   *httphandler.OAuthTokenHandler

	ContractsExportHandler *httphandler.ContractsExportHandler

	RegistryHandler *httphandler.RegistryHandler

	BundleReadUseCase     *bundle.BundleReadUseCase
	ControllerReadUseCase *registry.ControllerReadUseCase
	TenantReadUseCase     *registry.TenantReadUseCase
	BundleHTTPSyncUseCase *bundle.BundleHTTPSyncUseCase
	StatusReadUseCase     *registry.StatusReadUseCase

	ControllerRegistryHandler *grpchandler.ControllerRegistryHandler

	// BundleSyncIdempotency caches POST /v1/bundles/sync outcomes when Idempotency-Key is set (memory or etcd).
	BundleSyncIdempotency idempotency.Executor
}

// NewContainer creates and initializes a new DI container
func NewContainer(cfg *config.Config) (*Container, error) {
	if err := config.ValidateOIDCProviders(cfg.Auth.OIDCProviders); err != nil {
		return nil, err
	}
	if err := config.ValidateInteractiveTokenTTLPolicy(
		cfg.Auth.InteractiveAccessTokenTTL,
		cfg.Auth.InteractiveAccessTokenMaxTTL,
		cfg.Auth.InteractiveRefreshTokenTTL,
		cfg.Auth.InteractiveRefreshTokenMaxTTL,
	); err != nil {
		return nil, err
	}
	if err := config.ValidateAuthorizationConfig(cfg.Auth.Authorization); err != nil {
		return nil, err
	}
	if len(cfg.Auth.OIDCProviders) > 0 && strings.TrimSpace(cfg.Auth.SessionKEKBase64) == "" {
		return nil, fmt.Errorf("auth.session_kek_base64 is required when auth.oidc_providers is configured")
	}

	c := &Container{
		Config: cfg,
	}
	configuredRoles := make([]authzroles.ConfiguredRole, 0, len(cfg.Auth.Authorization.Roles))
	for i := range cfg.Auth.Authorization.Roles {
		r := cfg.Auth.Authorization.Roles[i]
		configuredRoles = append(configuredRoles, authzroles.ConfiguredRole{
			ID:          r.ID,
			Permissions: append([]string(nil), r.Permissions...),
		})
	}
	roleCatalog, err := authzroles.NewCatalog(configuredRoles)
	if err != nil {
		return nil, err
	}
	c.RoleCatalog = roleCatalog

	ok := false
	defer func() {
		if !ok {
			c.Close()
		}
	}()

	if err := c.initEtcd(); err != nil {
		return nil, err
	}
	c.initLeaderGate()
	c.initRepositories()
	if err := c.initUseCases(); err != nil {
		return nil, err
	}
	c.initHandlers()

	ok = true
	return c, nil
}

func (c *Container) initEtcd() error {
	client, err := bootstrap.ConnectEtcd(context.Background(), c.Config.Etcd, bootstrap.DefaultEtcdStatusTimeout)
	if err != nil {
		return err
	}
	c.EtcdClient = client
	return nil
}

func (c *Container) initLeaderGate() {
	le := c.Config.LeaderElection
	c.LeaderGate = bootstrap.StartLeaderElection(context.Background(), c.EtcdClient, bootstrap.LeaderElectionSettings{
		Enabled:           le.Enabled,
		Identity:          le.Identity,
		KeyPrefix:         le.KeyPrefix,
		SessionTTLSeconds: le.SessionTTLSeconds,
		DefaultKeyPrefix:  "/api-gateway/api-server/election/leader",
		FallbackIDPrefix:  "api-server",
		Service:           "api-server",
	})
}

func (c *Container) initRepositories() {
	c.SnapshotRepository = etcd.NewSnapshotRepository(c.EtcdClient)
	c.ControllerRepository = etcd.NewControllerRepository(c.EtcdClient)
	c.APIKeyRepository = etcd.NewAPIKeyRepository(c.EtcdClient, c.Config.Auth.EtcdKeyPrefix)
	c.SessionRepository = etcd.NewSessionRepository(c.EtcdClient, c.Config.Auth.EtcdKeyPrefix)
	c.LoginIntentRepository = etcd.NewLoginIntentRepository(c.EtcdClient, c.Config.Auth.EtcdKeyPrefix)

	slog.Info("repositories initialized")
}

func (c *Container) initUseCases() error {
	jwtUC, err := auth.NewJWTUseCase(&c.Config.JWT)
	if err != nil {
		return fmt.Errorf("initialize JWT use case: %w", err)
	}
	c.JWTUseCase = jwtUC

	if len(c.Config.Auth.OIDCProviders) > 0 {
		raw, decErr := base64.StdEncoding.DecodeString(strings.TrimSpace(c.Config.Auth.SessionKEKBase64))
		if decErr != nil || len(raw) != 32 {
			return fmt.Errorf("auth.session_kek_base64: need 32-byte AES-256 key (standard base64): %w", decErr)
		}
		kr, kerr := sessioncrypto.NewKeyring(sessioncrypto.KEK{ID: "default", Key: raw})
		sessioncrypto.ZeroBytes(raw)
		if kerr != nil {
			return fmt.Errorf("session KEK keyring: %w", kerr)
		}
		c.SessionSealer = kr
	}

	var idpAccessCache *idpcache.Cache
	if c.SessionSealer != nil && len(c.Config.Auth.OIDCProviders) > 0 {
		idpAccessCache = idpcache.New(nil)
		c.IdpAccessCache = idpAccessCache
		if c.Config.MetricsHTTP.Enabled {
			idpAccessCache.SetMetricsHooks(
				func(hit bool) {
					if hit {
						metrics.RecordIdpAccessCacheEvent(true, metrics.IdpAccessCacheHit)
					} else {
						metrics.RecordIdpAccessCacheEvent(true, metrics.IdpAccessCacheMiss)
					}
				},
				func() { metrics.RecordIdpAccessCacheEvent(true, metrics.IdpAccessCachePut) },
				func() { metrics.RecordIdpAccessCacheEvent(true, metrics.IdpAccessCacheInvalidate) },
			)
		}
	}

	c.OIDCLoginUseCase = auth.NewOIDCLoginUseCase(
		c.Config.Auth.OIDCProviders,
		c.Config.Auth.LoginIntentLeaseTTL,
		c.LoginIntentRepository,
		&http.Client{Timeout: 12 * time.Second},
		auth.TokenTTLPolicy{
			DefaultAccessTTL:  config.EffectiveInteractiveAccessTokenTTL(c.Config.Auth.InteractiveAccessTokenTTL),
			MaxAccessTTL:      config.EffectiveInteractiveAccessTokenMaxTTL(c.Config.Auth.InteractiveAccessTokenMaxTTL),
			DefaultRefreshTTL: config.EffectiveInteractiveRefreshTokenTTL(c.Config.Auth.InteractiveRefreshTokenTTL),
			MaxRefreshTTL:     config.EffectiveInteractiveRefreshTokenMaxTTL(c.Config.Auth.InteractiveRefreshTokenMaxTTL),
		},
	)

	c.OIDCCallbackUseCase = auth.NewOIDCCallbackUseCase(
		c.Config.Auth.OIDCProviders,
		c.LoginIntentRepository,
		c.SessionRepository,
		c.SessionSealer,
		c.JWTUseCase,
		&http.Client{Timeout: 20 * time.Second},
		auth.TokenTTLPolicy{
			DefaultAccessTTL:  config.EffectiveInteractiveAccessTokenTTL(c.Config.Auth.InteractiveAccessTokenTTL),
			MaxAccessTTL:      config.EffectiveInteractiveAccessTokenMaxTTL(c.Config.Auth.InteractiveAccessTokenMaxTTL),
			DefaultRefreshTTL: config.EffectiveInteractiveRefreshTokenTTL(c.Config.Auth.InteractiveRefreshTokenTTL),
			MaxRefreshTTL:     config.EffectiveInteractiveRefreshTokenMaxTTL(c.Config.Auth.InteractiveRefreshTokenMaxTTL),
		},
		idpAccessCache,
		c.Config.Auth.IdpAccessCacheOpaqueMaxTTL,
	)

	if len(c.Config.Auth.OIDCProviders) > 0 && c.SessionSealer != nil {
		c.OIDCRefreshUseCase = auth.NewOIDCRefreshUseCase(
			c.Config.Auth.OIDCProviders,
			c.SessionRepository,
			c.SessionSealer,
			c.JWTUseCase,
			&http.Client{Timeout: 25 * time.Second},
			auth.TokenTTLPolicy{
				DefaultAccessTTL:  config.EffectiveInteractiveAccessTokenTTL(c.Config.Auth.InteractiveAccessTokenTTL),
				MaxAccessTTL:      config.EffectiveInteractiveAccessTokenMaxTTL(c.Config.Auth.InteractiveAccessTokenMaxTTL),
				DefaultRefreshTTL: config.EffectiveInteractiveRefreshTokenTTL(c.Config.Auth.InteractiveRefreshTokenTTL),
				MaxRefreshTTL:     config.EffectiveInteractiveRefreshTokenMaxTTL(c.Config.Auth.InteractiveRefreshTokenMaxTTL),
			},
			c.Config.MetricsHTTP.Enabled,
			idpAccessCache,
			c.Config.Auth.IdpAccessCacheOpaqueMaxTTL,
		)
	}
	if len(c.Config.Auth.OIDCProviders) > 0 && c.SessionSealer != nil {
		c.OAuthTokenUseCase = auth.NewOAuthTokenUseCase(
			c.LoginIntentRepository,
			c.SessionRepository,
			c.JWTUseCase,
			c.OIDCRefreshUseCase,
			auth.TokenTTLPolicy{
				DefaultAccessTTL:  config.EffectiveInteractiveAccessTokenTTL(c.Config.Auth.InteractiveAccessTokenTTL),
				MaxAccessTTL:      config.EffectiveInteractiveAccessTokenMaxTTL(c.Config.Auth.InteractiveAccessTokenMaxTTL),
				DefaultRefreshTTL: config.EffectiveInteractiveRefreshTokenTTL(c.Config.Auth.InteractiveRefreshTokenTTL),
				MaxRefreshTTL:     config.EffectiveInteractiveRefreshTokenMaxTTL(c.Config.Auth.InteractiveRefreshTokenMaxTTL),
			},
		)
	}

	c.ControllerRegistryUseCase = registry.NewControllerRegistryUseCase(
		c.ControllerRepository,
		c.SnapshotRepository,
		c.EtcdClient,
	)

	c.ContractSyncerGRPC = contractsyncergrpc.NewClient(c.Config.ContractSyncer.Address, c.Config.GRPCContractSyncerClient)

	c.BundleSyncUseCase = bundle.NewBundleSyncUseCase(
		c.SnapshotRepository,
		c.ControllerRepository,
		c.ContractSyncerGRPC,
		c.LeaderGate,
		c.Config.MetricsHTTP.Enabled,
	)

	c.BundleReadUseCase = bundle.NewBundleReadUseCase(c.SnapshotRepository)
	c.ControllerReadUseCase = registry.NewControllerReadUseCase(c.ControllerRepository)
	c.TenantReadUseCase = registry.NewTenantReadUseCase(c.ControllerRepository)
	c.BundleHTTPSyncUseCase = bundle.NewBundleHTTPSyncUseCase(c.SnapshotRepository, c.BundleSyncUseCase)
	c.StatusReadUseCase = registry.NewStatusReadUseCase(
		c.EtcdClient,
		c.ContractSyncerGRPC,
	)

	slog.Info("use cases initialized")
	return nil
}

func (c *Container) initHandlers() {
	switch strings.ToLower(strings.TrimSpace(c.Config.Idempotency.Backend)) {
	case "etcd":
		pfx := idempotency.ResolveKeyPrefix(c.Config.Idempotency.EtcdKeyPrefix, c.Config.Idempotency.Cluster)
		c.BundleSyncIdempotency = idempotency.NewEtcdStore(c.EtcdClient, pfx, c.Config.Idempotency.BundleSyncTTL)
		slog.Info("idempotency backend=etcd", "etcd_key_prefix", pfx)
	default:
		c.BundleSyncIdempotency = idempotency.NewStore(c.Config.Idempotency.BundleSyncTTL)
		slog.Info("idempotency backend=memory")
	}
	c.PermissionEvaluator = httpauthz.NewPermissionEvaluator(c.RoleCatalog)
	c.JWTHandler = httphandler.NewJWTHandler(
		c.JWTUseCase,
		c.Config.MetricsHTTP.Enabled,
		c.Config.Auth.InteractiveAccessTokenTTL,
		c.PermissionEvaluator,
	)
	c.OIDCLoginHandler = httphandler.NewOIDCLoginHandler(c.OIDCLoginUseCase)
	c.OIDCCallbackHandler = httphandler.NewOIDCCallbackHandler(c.OIDCCallbackUseCase)
	if c.OAuthTokenUseCase != nil {
		c.OAuthTokenHandler = httphandler.NewOAuthTokenHandler(c.OAuthTokenUseCase)
	}
	exportUC := bundle.NewContractExportUseCase(c.ContractSyncerGRPC)
	c.ContractsExportHandler = httphandler.NewContractsExportHandler(exportUC, c.PermissionEvaluator)
	c.RegistryHandler = httphandler.NewRegistryHandler(
		c.BundleReadUseCase,
		c.ControllerReadUseCase,
		c.TenantReadUseCase,
		c.BundleHTTPSyncUseCase,
		c.StatusReadUseCase,
		c.Config.Readiness.RequireContractSyncer,
		c.BundleSyncIdempotency,
	)
	c.ControllerRegistryHandler = grpchandler.NewControllerRegistryHandler(c.ControllerRegistryUseCase, c.Config.MetricsHTTP.Enabled)

	slog.Info("handlers initialized")
}

// StartBackgroundWork runs etcd watch and bundle sync until ctx is cancelled.
func (c *Container) StartBackgroundWork(ctx context.Context) {
	go c.ControllerRegistryUseCase.StartEtcdWatch(ctx)
	go c.BundleSyncUseCase.StartBundleWatcher(ctx)
	slog.Info("API server etcd watch and bundle watcher started")
}

// Close closes all resources in the container
func (c *Container) Close() {
	if c.IdpAccessCache != nil {
		c.IdpAccessCache.Close()
		c.IdpAccessCache = nil
	}
	if c.SessionSealer != nil {
		c.SessionSealer.Close()
		c.SessionSealer = nil
	}
	bootstrap.CloseEtcdClient(c.EtcdClient)
}
