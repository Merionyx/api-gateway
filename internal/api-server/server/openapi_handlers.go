package server

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/container"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/version"
)

// OpenAPIServer implements apiserver.ServerInterface by delegating to HTTP handlers from the DI container.
type OpenAPIServer struct {
	c *container.Container
}

// NewOpenAPIServer returns a ServerInterface backed by the DI container.
func NewOpenAPIServer(c *container.Container) apiserver.ServerInterface {
	return &OpenAPIServer{c: c}
}

func (s *OpenAPIServer) GetJwks(c fiber.Ctx, _ apiserver.GetJwksParams) error {
	return s.c.JWTHandler.GetJWKS(c)
}

func (s *OpenAPIServer) GetJwksEdge(c fiber.Ctx, _ apiserver.GetJwksEdgeParams) error {
	return s.c.JWTHandler.GetJWKSEdge(c)
}

func (s *OpenAPIServer) ListOidcProviders(c fiber.Ctx) error {
	return s.c.OIDCLoginHandler.ListOidcProviders(c)
}

func (s *OpenAPIServer) AuthorizeOidc(c fiber.Ctx, params apiserver.AuthorizeOidcParams) error {
	return s.c.OIDCLoginHandler.Authorize(c, params)
}

func (s *OpenAPIServer) CallbackOidc(c fiber.Ctx, params apiserver.CallbackOidcParams) error {
	return s.c.OIDCCallbackHandler.Callback(c, params)
}

func (s *OpenAPIServer) TokenOidc(c fiber.Ctx) error {
	if s.c.OAuthTokenHandler == nil {
		return authFlowNotImplemented(c, "OAuth token endpoint requires auth.oidc_providers and auth.session_kek_base64.")
	}
	return s.c.OAuthTokenHandler.Token(c)
}

func (s *OpenAPIServer) IssueEdgeToken(c fiber.Ctx) error {
	return s.c.JWTHandler.GenerateToken(c)
}

func (s *OpenAPIServer) IssueApiAccessToken(c fiber.Ctx) error {
	return s.c.JWTHandler.IssueApiAccessToken(c)
}

func authFlowNotImplemented(c fiber.Ctx, detail string) error {
	return problem.Write(c, http.StatusNotImplemented, problem.WithCode(
		http.StatusNotImplemented,
		"FEATURE_NOT_IMPLEMENTED",
		"Not Implemented",
		detail,
	))
}

func (s *OpenAPIServer) ListBundleKeys(c fiber.Ctx, params apiserver.ListBundleKeysParams) error {
	return s.c.RegistryHandler.ListBundleKeys(c, params)
}

func (s *OpenAPIServer) SyncBundle(c fiber.Ctx, params apiserver.SyncBundleParams) error {
	return s.c.RegistryHandler.SyncBundle(c, params)
}

func (s *OpenAPIServer) ListContractsInBundle(c fiber.Ctx, params apiserver.ListContractsInBundleParams) error {
	return s.c.RegistryHandler.ListContractsInBundle(c, params)
}

func (s *OpenAPIServer) GetContractInBundle(c fiber.Ctx, contractName apiserver.ContractName, params apiserver.GetContractInBundleParams) error {
	return s.c.RegistryHandler.GetContractInBundle(c, contractName, params)
}

func (s *OpenAPIServer) ExportContracts(c fiber.Ctx) error {
	return s.c.ContractsExportHandler.Export(c)
}

func (s *OpenAPIServer) ListControllers(c fiber.Ctx, params apiserver.ListControllersParams) error {
	return s.c.RegistryHandler.ListControllers(c, params)
}

func (s *OpenAPIServer) GetController(c fiber.Ctx, controllerId apiserver.ControllerId, params apiserver.GetControllerParams) error {
	return s.c.RegistryHandler.GetController(c, controllerId, params)
}

func (s *OpenAPIServer) GetControllerHeartbeat(c fiber.Ctx, controllerId apiserver.ControllerId, params apiserver.GetControllerHeartbeatParams) error {
	return s.c.RegistryHandler.GetControllerHeartbeat(c, controllerId, params)
}

func (s *OpenAPIServer) ListSigningKeys(c fiber.Ctx, _ apiserver.ListSigningKeysParams) error {
	return s.c.JWTHandler.GetSigningKeys(c)
}

func (s *OpenAPIServer) GetStatus(c fiber.Ctx, params apiserver.GetStatusParams) error {
	return s.c.RegistryHandler.GetStatus(c, params)
}

func (s *OpenAPIServer) ListTenants(c fiber.Ctx, params apiserver.ListTenantsParams) error {
	return s.c.RegistryHandler.ListTenants(c, params)
}

func (s *OpenAPIServer) ListBundlesByTenant(c fiber.Ctx, tenant apiserver.Tenant, params apiserver.ListBundlesByTenantParams) error {
	return s.c.RegistryHandler.ListBundlesByTenant(c, tenant, params)
}

func (s *OpenAPIServer) ListControllersByTenant(c fiber.Ctx, tenant apiserver.Tenant, params apiserver.ListControllersByTenantParams) error {
	return s.c.RegistryHandler.ListControllersByTenant(c, tenant, params)
}

func (s *OpenAPIServer) ListEnvironmentsByTenant(c fiber.Ctx, tenant apiserver.Tenant, params apiserver.ListEnvironmentsByTenantParams) error {
	return s.c.RegistryHandler.ListEnvironmentsByTenant(c, tenant, params)
}

func (s *OpenAPIServer) GetVersion(c fiber.Ctx) error {
	body := apiserver.VersionResponse{
		ApiSchemaVersion: version.APISchemaVersion(),
		GitRevision:      version.GitRevision,
		BuildTime:        version.BuildTime,
	}
	if version.Release != "" {
		r := version.Release
		body.Release = &r
	}
	return c.JSON(body)
}

func (s *OpenAPIServer) GetHealth(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

func (s *OpenAPIServer) GetReady(c fiber.Ctx) error {
	return s.c.RegistryHandler.GetReady(c)
}
