package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/container"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/version"
)

type strictFiberCtxKey struct{}

func bindFiberContextForStrictHandlers() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.SetContext(context.WithValue(c.Context(), strictFiberCtxKey{}, c))
		return c.Next()
	}
}

func fiberCtxFromStrictContext(ctx context.Context) (fiber.Ctx, error) {
	if ctx == nil {
		return nil, fmt.Errorf("missing strict request context")
	}
	fc, ok := ctx.Value(strictFiberCtxKey{}).(fiber.Ctx)
	if !ok || fc == nil {
		return nil, fmt.Errorf("missing fiber context in strict request context")
	}
	return fc, nil
}

// StrictOpenAPIServer implements generated StrictServerInterface and delegates execution
// directly to concrete HTTP handlers in the DI container.
type StrictOpenAPIServer struct {
	c *container.Container
}

func NewStrictOpenAPIServer(c *container.Container) apiserver.StrictServerInterface {
	return &StrictOpenAPIServer{
		c: c,
	}
}

func (s *StrictOpenAPIServer) GetJwksEdge(ctx context.Context, request apiserver.GetJwksEdgeRequestObject) (apiserver.GetJwksEdgeResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.JWTHandler.GetJWKSEdge(c)
}

func (s *StrictOpenAPIServer) GetJwks(ctx context.Context, request apiserver.GetJwksRequestObject) (apiserver.GetJwksResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.JWTHandler.GetJWKS(c)
}

func (s *StrictOpenAPIServer) GetHealth(ctx context.Context, _ apiserver.GetHealthRequestObject) (apiserver.GetHealthResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, c.JSON(fiber.Map{"status": "ok"})
}

func (s *StrictOpenAPIServer) GetReady(ctx context.Context, _ apiserver.GetReadyRequestObject) (apiserver.GetReadyResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.RegistryHandler.GetReady(c)
}

func (s *StrictOpenAPIServer) AuthorizeOidc(ctx context.Context, request apiserver.AuthorizeOidcRequestObject) (apiserver.AuthorizeOidcResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.OIDCLoginHandler.Authorize(c, request.Params)
}

func (s *StrictOpenAPIServer) CallbackOidc(ctx context.Context, request apiserver.CallbackOidcRequestObject) (apiserver.CallbackOidcResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.OIDCCallbackHandler.Callback(c, request.Params)
}

func (s *StrictOpenAPIServer) ListOidcProviders(ctx context.Context, _ apiserver.ListOidcProvidersRequestObject) (apiserver.ListOidcProvidersResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.OIDCLoginHandler.ListOidcProviders(c)
}

func (s *StrictOpenAPIServer) ListAuthPermissions(ctx context.Context, _ apiserver.ListAuthPermissionsRequestObject) (apiserver.ListAuthPermissionsResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.AuthIntrospectionHandler.ListPermissions(c)
}

func (s *StrictOpenAPIServer) ListAuthRoles(ctx context.Context, _ apiserver.ListAuthRolesRequestObject) (apiserver.ListAuthRolesResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.AuthIntrospectionHandler.ListRoles(c)
}

func (s *StrictOpenAPIServer) TokenOidc(ctx context.Context, _ apiserver.TokenOidcRequestObject) (apiserver.TokenOidcResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	if s.c.OAuthTokenHandler == nil {
		return nil, problem.Write(c, http.StatusNotImplemented, problem.WithCode(
			http.StatusNotImplemented,
			"FEATURE_NOT_IMPLEMENTED",
			"Not Implemented",
			"OAuth token endpoint requires auth.oidc_providers and auth.session_kek_base64.",
		))
	}
	return nil, s.c.OAuthTokenHandler.Token(c)
}

func (s *StrictOpenAPIServer) InspectTokenPermissions(ctx context.Context, _ apiserver.InspectTokenPermissionsRequestObject) (apiserver.InspectTokenPermissionsResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.AuthIntrospectionHandler.InspectTokenPermissions(c)
}

func (s *StrictOpenAPIServer) ListBundleKeys(ctx context.Context, request apiserver.ListBundleKeysRequestObject) (apiserver.ListBundleKeysResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.RegistryHandler.ListBundleKeys(c, request.Params)
}

func (s *StrictOpenAPIServer) ListContractsInBundle(ctx context.Context, request apiserver.ListContractsInBundleRequestObject) (apiserver.ListContractsInBundleResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.RegistryHandler.ListContractsInBundle(c, request.Params)
}

func (s *StrictOpenAPIServer) GetContractInBundle(ctx context.Context, request apiserver.GetContractInBundleRequestObject) (apiserver.GetContractInBundleResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.RegistryHandler.GetContractInBundle(c, request.ContractName, request.Params)
}

func (s *StrictOpenAPIServer) SyncBundle(ctx context.Context, request apiserver.SyncBundleRequestObject) (apiserver.SyncBundleResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.RegistryHandler.SyncBundle(c, request.Params)
}

func (s *StrictOpenAPIServer) ExportContracts(ctx context.Context, _ apiserver.ExportContractsRequestObject) (apiserver.ExportContractsResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.ContractsExportHandler.Export(c)
}

func (s *StrictOpenAPIServer) ListControllers(ctx context.Context, request apiserver.ListControllersRequestObject) (apiserver.ListControllersResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.RegistryHandler.ListControllers(c, request.Params)
}

func (s *StrictOpenAPIServer) GetController(ctx context.Context, request apiserver.GetControllerRequestObject) (apiserver.GetControllerResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.RegistryHandler.GetController(c, request.ControllerId, request.Params)
}

func (s *StrictOpenAPIServer) GetControllerHeartbeat(ctx context.Context, request apiserver.GetControllerHeartbeatRequestObject) (apiserver.GetControllerHeartbeatResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.RegistryHandler.GetControllerHeartbeat(c, request.ControllerId, request.Params)
}

func (s *StrictOpenAPIServer) ListSigningKeys(ctx context.Context, request apiserver.ListSigningKeysRequestObject) (apiserver.ListSigningKeysResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.JWTHandler.GetSigningKeys(c)
}

func (s *StrictOpenAPIServer) GetStatus(ctx context.Context, request apiserver.GetStatusRequestObject) (apiserver.GetStatusResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.RegistryHandler.GetStatus(c, request.Params)
}

func (s *StrictOpenAPIServer) ListTenants(ctx context.Context, request apiserver.ListTenantsRequestObject) (apiserver.ListTenantsResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.RegistryHandler.ListTenants(c, request.Params)
}

func (s *StrictOpenAPIServer) ListBundlesByTenant(ctx context.Context, request apiserver.ListBundlesByTenantRequestObject) (apiserver.ListBundlesByTenantResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.RegistryHandler.ListBundlesByTenant(c, request.Tenant, request.Params)
}

func (s *StrictOpenAPIServer) ListControllersByTenant(ctx context.Context, request apiserver.ListControllersByTenantRequestObject) (apiserver.ListControllersByTenantResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.RegistryHandler.ListControllersByTenant(c, request.Tenant, request.Params)
}

func (s *StrictOpenAPIServer) ListEnvironmentsByTenant(ctx context.Context, request apiserver.ListEnvironmentsByTenantRequestObject) (apiserver.ListEnvironmentsByTenantResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.RegistryHandler.ListEnvironmentsByTenant(c, request.Tenant, request.Params)
}

func (s *StrictOpenAPIServer) IssueApiAccessToken(ctx context.Context, _ apiserver.IssueApiAccessTokenRequestObject) (apiserver.IssueApiAccessTokenResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.JWTHandler.IssueApiAccessToken(c)
}

func (s *StrictOpenAPIServer) IssueEdgeToken(ctx context.Context, _ apiserver.IssueEdgeTokenRequestObject) (apiserver.IssueEdgeTokenResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.c.JWTHandler.GenerateToken(c)
}

func (s *StrictOpenAPIServer) GetVersion(ctx context.Context, _ apiserver.GetVersionRequestObject) (apiserver.GetVersionResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	body := apiserver.VersionResponse{
		ApiSchemaVersion: version.APISchemaVersion(),
		GitRevision:      version.GitRevision,
		BuildTime:        version.BuildTime,
	}
	if version.Release != "" {
		r := version.Release
		body.Release = &r
	}
	return nil, c.JSON(body)
}

var _ apiserver.StrictServerInterface = (*StrictOpenAPIServer)(nil)
