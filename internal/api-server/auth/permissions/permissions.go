package permissions

const (
	// Wildcard grants access to all permissions.
	Wildcard = "*"

	// Token and contract permissions used by protected API Server operations.
	EdgeTokenIssue      = "api.token.edge.issue"
	APIAccessTokenIssue = "api.token.api.issue"
	ContractsExport     = "api.contracts.export"

	// Read-only baseline permissions for viewer-style access.
	StatusRead     = "api.status.read"
	RegistryRead   = "api.registry.read"
	BundleRead     = "api.bundle.read"
	ControllerRead = "api.controllers.read"
	TenantRead     = "api.tenants.read"
)
