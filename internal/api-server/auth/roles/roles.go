// Package roles defines stable API Server role strings used by RBAC/ABAC authorization.
package roles

const (
	// APIRoleAdmin is the immutable built-in admin role (full access).
	APIRoleAdmin = "api:role:admin"
	// APIRoleViewer is the immutable built-in read-only baseline role.
	APIRoleViewer = "api:role:viewer"

	// APIEdgeTokensIssue allows POST /v1/tokens/edge.
	APIEdgeTokensIssue = "api:edge_tokens:issue" // #nosec G101 -- role identifier, not a credential.
	// APIAccessTokensIssue allows POST /v1/tokens/api (M2M or delegated interactive).
	APIAccessTokensIssue = "api:access_tokens:issue"
	// APIContractsExport allows POST /v1/contracts/export.
	APIContractsExport = "api:contracts:export"
)
