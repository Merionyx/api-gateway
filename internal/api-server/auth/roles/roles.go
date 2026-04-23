// Package roles defines stable API Server role strings for RBAC (roadmap ш. 23).
// CEL at login/refresh (IdP up) will populate or replace the `roles` claim; until then,
// interactive sessions use a baseline role (see usecase auth snapshot helpers).
package roles

const (
	// APIMember is the default interactive baseline: authenticated API callers without extra privileges.
	APIMember = "api:member"
	// APIAdmin grants all in-process RBAC checks (use sparingly; prefer narrow roles).
	APIAdmin = "api:admin"
	// APIAccessTokensIssue allows POST /api/v1/tokens/api (M2M or delegated interactive).
	APIAccessTokensIssue = "api:access_tokens:issue"
	// APIContractsExport allows POST /api/v1/contracts/export.
	APIContractsExport = "api:contracts:export"
)
