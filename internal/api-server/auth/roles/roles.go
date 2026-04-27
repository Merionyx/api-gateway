// Package roles defines stable API Server role strings used by RBAC/ABAC authorization.
package roles

const (
	// APIRoleAdmin is the immutable built-in admin role (full access).
	APIRoleAdmin = "api:role:admin"
	// APIRoleViewer is the immutable built-in read-only baseline role.
	APIRoleViewer = "api:role:viewer"

	// APIEdgeTokensIssue allows POST /api/v1/tokens/edge.
	APIEdgeTokensIssue = "api:edge_tokens:issue"
	// APIAccessTokensIssue allows POST /api/v1/tokens/api (M2M or delegated interactive).
	APIAccessTokensIssue = "api:access_tokens:issue"
	// APIContractsExport allows POST /api/v1/contracts/export.
	APIContractsExport = "api:contracts:export"
)
