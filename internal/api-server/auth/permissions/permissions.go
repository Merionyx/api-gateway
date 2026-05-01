package permissions

import "sort"

const (
	// Wildcard grants access to all permissions.
	Wildcard = "*"

	// Token and contract permissions used by protected API Server operations.
	EdgeTokenIssue      = "api.token.edge.issue" // #nosec G101 -- permission identifier, not a credential.
	APIAccessTokenIssue = "api.token.api.issue"  // #nosec G101 -- permission identifier, not a credential.
	ContractsExport     = "api.contracts.export"

	// Read-only baseline permissions for viewer-style access.
	StatusRead     = "api.status.read"
	RegistryRead   = "api.registry.read"
	BundleRead     = "api.bundle.read"
	ControllerRead = "api.controllers.read"
	TenantRead     = "api.tenants.read"
)

const unknownPermissionDescription = "Custom permission from configuration or token claims."

var descriptionsByID = map[string]string{ // #nosec G101 -- static permission descriptions, not secrets.
	Wildcard:            "Wildcard permission granting access to all operations.",
	EdgeTokenIssue:      "Issue Edge-profile JWT tokens for data plane and ExtAuthz.",
	APIAccessTokenIssue: "Issue API-profile access JWT tokens for calling API Server HTTP routes.",
	ContractsExport:     "Export contracts from configured repositories via Contract Syncer.",
	StatusRead:          "Read aggregate API Server status and readiness information.",
	RegistryRead:        "Read registry entities and metadata from control-plane snapshots.",
	BundleRead:          "Read bundle lists and contracts from stored snapshots.",
	ControllerRead:      "Read controller registration, heartbeat, and environment views.",
	TenantRead:          "Read tenant-level views and related registry projections.",
}

// Descriptor is a documented permission entry.
type Descriptor struct {
	ID          string
	Description string
}

// Describe returns a human-readable description for a permission id.
// Unknown/custom permission ids return a generic description.
func Describe(id string) string {
	if s, ok := descriptionsByID[id]; ok {
		return s
	}
	return unknownPermissionDescription
}

// ListDescriptors returns built-in permissions with stable ordering by id.
func ListDescriptors() []Descriptor {
	keys := make([]string, 0, len(descriptionsByID))
	for id := range descriptionsByID {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	out := make([]Descriptor, 0, len(keys))
	for _, id := range keys {
		out = append(out, Descriptor{
			ID:          id,
			Description: descriptionsByID[id],
		})
	}
	return out
}
