package server

import (
	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/container"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
)

// OpenAPIServer implements apiserver.ServerInterface by delegating to existing HTTP handlers
// where implemented; other operations return 501 until wired.
type OpenAPIServer struct {
	c *container.Container
}

// NewOpenAPIServer returns a ServerInterface backed by the DI container.
func NewOpenAPIServer(c *container.Container) apiserver.ServerInterface {
	return &OpenAPIServer{c: c}
}

func notImplemented(c fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"error": "not implemented",
	})
}

func (s *OpenAPIServer) GetJwks(c fiber.Ctx, _ apiserver.GetJwksParams) error {
	return s.c.JWTHandler.GetJWKS(c)
}

func (s *OpenAPIServer) ListBundleKeys(c fiber.Ctx, _ apiserver.ListBundleKeysParams) error {
	return notImplemented(c)
}

func (s *OpenAPIServer) SyncBundle(c fiber.Ctx, _ apiserver.SyncBundleParams) error {
	return notImplemented(c)
}

func (s *OpenAPIServer) ListContractsInBundle(c fiber.Ctx, _ apiserver.BundleKey, _ apiserver.ListContractsInBundleParams) error {
	return notImplemented(c)
}

func (s *OpenAPIServer) GetContractInBundle(c fiber.Ctx, _ apiserver.BundleKey, _ apiserver.ContractName, _ apiserver.GetContractInBundleParams) error {
	return notImplemented(c)
}

func (s *OpenAPIServer) ExportContracts(c fiber.Ctx) error {
	return s.c.ContractsExportHandler.Export(c)
}

func (s *OpenAPIServer) ListControllers(c fiber.Ctx, _ apiserver.ListControllersParams) error {
	return notImplemented(c)
}

func (s *OpenAPIServer) GetController(c fiber.Ctx, _ apiserver.ControllerId, _ apiserver.GetControllerParams) error {
	return notImplemented(c)
}

func (s *OpenAPIServer) GetControllerHeartbeat(c fiber.Ctx, _ apiserver.ControllerId, _ apiserver.GetControllerHeartbeatParams) error {
	return notImplemented(c)
}

func (s *OpenAPIServer) ListSigningKeys(c fiber.Ctx, _ apiserver.ListSigningKeysParams) error {
	return s.c.JWTHandler.GetSigningKeys(c)
}

func (s *OpenAPIServer) GetStatus(c fiber.Ctx, _ apiserver.GetStatusParams) error {
	return notImplemented(c)
}

func (s *OpenAPIServer) ListTenants(c fiber.Ctx, _ apiserver.ListTenantsParams) error {
	return notImplemented(c)
}

func (s *OpenAPIServer) ListBundlesByTenant(c fiber.Ctx, _ apiserver.Tenant, _ apiserver.ListBundlesByTenantParams) error {
	return notImplemented(c)
}

func (s *OpenAPIServer) ListControllersByTenant(c fiber.Ctx, _ apiserver.Tenant, _ apiserver.ListControllersByTenantParams) error {
	return notImplemented(c)
}

func (s *OpenAPIServer) ListEnvironmentsByTenant(c fiber.Ctx, _ apiserver.Tenant, _ apiserver.ListEnvironmentsByTenantParams) error {
	return notImplemented(c)
}

func (s *OpenAPIServer) CreateToken(c fiber.Ctx) error {
	return s.c.JWTHandler.GenerateToken(c)
}

func (s *OpenAPIServer) GetHealth(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}
