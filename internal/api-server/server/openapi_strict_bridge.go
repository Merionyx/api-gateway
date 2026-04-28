package server

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/container"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
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
// to the current OpenAPI handlers wired through ServerInterface implementation.
type StrictOpenAPIServer struct {
	legacy apiserver.ServerInterface
}

func NewStrictOpenAPIServer(c *container.Container) apiserver.StrictServerInterface {
	return &StrictOpenAPIServer{
		legacy: NewOpenAPIServer(c),
	}
}

func (s *StrictOpenAPIServer) GetJwksEdge(ctx context.Context, request apiserver.GetJwksEdgeRequestObject) (apiserver.GetJwksEdgeResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.GetJwksEdge(c, request.Params)
}

func (s *StrictOpenAPIServer) GetJwks(ctx context.Context, request apiserver.GetJwksRequestObject) (apiserver.GetJwksResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.GetJwks(c, request.Params)
}

func (s *StrictOpenAPIServer) GetHealth(ctx context.Context, _ apiserver.GetHealthRequestObject) (apiserver.GetHealthResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.GetHealth(c)
}

func (s *StrictOpenAPIServer) GetReady(ctx context.Context, _ apiserver.GetReadyRequestObject) (apiserver.GetReadyResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.GetReady(c)
}

func (s *StrictOpenAPIServer) AuthorizeOidc(ctx context.Context, request apiserver.AuthorizeOidcRequestObject) (apiserver.AuthorizeOidcResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.AuthorizeOidc(c, request.Params)
}

func (s *StrictOpenAPIServer) CallbackOidc(ctx context.Context, request apiserver.CallbackOidcRequestObject) (apiserver.CallbackOidcResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.CallbackOidc(c, request.Params)
}

func (s *StrictOpenAPIServer) ListOidcProviders(ctx context.Context, _ apiserver.ListOidcProvidersRequestObject) (apiserver.ListOidcProvidersResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.ListOidcProviders(c)
}

func (s *StrictOpenAPIServer) ListAuthPermissions(ctx context.Context, _ apiserver.ListAuthPermissionsRequestObject) (apiserver.ListAuthPermissionsResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.ListAuthPermissions(c)
}

func (s *StrictOpenAPIServer) ListAuthRoles(ctx context.Context, _ apiserver.ListAuthRolesRequestObject) (apiserver.ListAuthRolesResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.ListAuthRoles(c)
}

func (s *StrictOpenAPIServer) TokenOidc(ctx context.Context, _ apiserver.TokenOidcRequestObject) (apiserver.TokenOidcResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.TokenOidc(c)
}

func (s *StrictOpenAPIServer) InspectTokenPermissions(ctx context.Context, _ apiserver.InspectTokenPermissionsRequestObject) (apiserver.InspectTokenPermissionsResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.InspectTokenPermissions(c)
}

func (s *StrictOpenAPIServer) ListBundleKeys(ctx context.Context, request apiserver.ListBundleKeysRequestObject) (apiserver.ListBundleKeysResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.ListBundleKeys(c, request.Params)
}

func (s *StrictOpenAPIServer) ListContractsInBundle(ctx context.Context, request apiserver.ListContractsInBundleRequestObject) (apiserver.ListContractsInBundleResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.ListContractsInBundle(c, request.Params)
}

func (s *StrictOpenAPIServer) GetContractInBundle(ctx context.Context, request apiserver.GetContractInBundleRequestObject) (apiserver.GetContractInBundleResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.GetContractInBundle(c, request.ContractName, request.Params)
}

func (s *StrictOpenAPIServer) SyncBundle(ctx context.Context, request apiserver.SyncBundleRequestObject) (apiserver.SyncBundleResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.SyncBundle(c, request.Params)
}

func (s *StrictOpenAPIServer) ExportContracts(ctx context.Context, _ apiserver.ExportContractsRequestObject) (apiserver.ExportContractsResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.ExportContracts(c)
}

func (s *StrictOpenAPIServer) ListControllers(ctx context.Context, request apiserver.ListControllersRequestObject) (apiserver.ListControllersResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.ListControllers(c, request.Params)
}

func (s *StrictOpenAPIServer) GetController(ctx context.Context, request apiserver.GetControllerRequestObject) (apiserver.GetControllerResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.GetController(c, request.ControllerId, request.Params)
}

func (s *StrictOpenAPIServer) GetControllerHeartbeat(ctx context.Context, request apiserver.GetControllerHeartbeatRequestObject) (apiserver.GetControllerHeartbeatResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.GetControllerHeartbeat(c, request.ControllerId, request.Params)
}

func (s *StrictOpenAPIServer) ListSigningKeys(ctx context.Context, request apiserver.ListSigningKeysRequestObject) (apiserver.ListSigningKeysResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.ListSigningKeys(c, request.Params)
}

func (s *StrictOpenAPIServer) GetStatus(ctx context.Context, request apiserver.GetStatusRequestObject) (apiserver.GetStatusResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.GetStatus(c, request.Params)
}

func (s *StrictOpenAPIServer) ListTenants(ctx context.Context, request apiserver.ListTenantsRequestObject) (apiserver.ListTenantsResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.ListTenants(c, request.Params)
}

func (s *StrictOpenAPIServer) ListBundlesByTenant(ctx context.Context, request apiserver.ListBundlesByTenantRequestObject) (apiserver.ListBundlesByTenantResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.ListBundlesByTenant(c, request.Tenant, request.Params)
}

func (s *StrictOpenAPIServer) ListControllersByTenant(ctx context.Context, request apiserver.ListControllersByTenantRequestObject) (apiserver.ListControllersByTenantResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.ListControllersByTenant(c, request.Tenant, request.Params)
}

func (s *StrictOpenAPIServer) ListEnvironmentsByTenant(ctx context.Context, request apiserver.ListEnvironmentsByTenantRequestObject) (apiserver.ListEnvironmentsByTenantResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.ListEnvironmentsByTenant(c, request.Tenant, request.Params)
}

func (s *StrictOpenAPIServer) IssueApiAccessToken(ctx context.Context, _ apiserver.IssueApiAccessTokenRequestObject) (apiserver.IssueApiAccessTokenResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.IssueApiAccessToken(c)
}

func (s *StrictOpenAPIServer) IssueEdgeToken(ctx context.Context, _ apiserver.IssueEdgeTokenRequestObject) (apiserver.IssueEdgeTokenResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.IssueEdgeToken(c)
}

func (s *StrictOpenAPIServer) GetVersion(ctx context.Context, _ apiserver.GetVersionRequestObject) (apiserver.GetVersionResponseObject, error) {
	c, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	return nil, s.legacy.GetVersion(c)
}

var _ apiserver.StrictServerInterface = (*StrictOpenAPIServer)(nil)
