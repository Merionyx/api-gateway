package roles

import (
	"fmt"
	"sort"
	"strings"

	"github.com/merionyx/api-gateway/internal/api-server/auth/permissions"
)

// ConfiguredRole defines one role from config with an explicit permission set.
type ConfiguredRole struct {
	ID          string
	Permissions []string
}

// Catalog resolves role IDs into effective permissions.
type Catalog struct {
	permissionsByRole map[string]map[string]struct{}
}

// RolePermissions is one role with its effective permission ids.
type RolePermissions struct {
	RoleID      string
	Permissions []string
}

// ImmutableRoleIDs returns built-in role IDs that cannot be overridden by config.
func ImmutableRoleIDs() []string {
	return []string{APIRoleAdmin, APIRoleViewer}
}

// NewCatalog builds a role catalog from immutable built-ins and configured roles.
func NewCatalog(configured []ConfiguredRole) (*Catalog, error) {
	c := &Catalog{permissionsByRole: make(map[string]map[string]struct{})}
	for roleID, perms := range builtInRoles() {
		c.permissionsByRole[roleID] = toPermissionSet(perms)
	}
	for i := range configured {
		roleID := strings.TrimSpace(configured[i].ID)
		if roleID == "" {
			return nil, fmt.Errorf("auth.authorization.roles[%d].id is required", i)
		}
		if _, exists := c.permissionsByRole[roleID]; exists {
			return nil, fmt.Errorf("auth.authorization.roles[%q]: role id is already defined", roleID)
		}
		perms := normalizePermissionList(configured[i].Permissions)
		if len(perms) == 0 {
			return nil, fmt.Errorf("auth.authorization.roles[%q]: permissions must be non-empty", roleID)
		}
		c.permissionsByRole[roleID] = toPermissionSet(perms)
	}
	return c, nil
}

// ResolvePermissions expands role IDs into a deduplicated permission set.
func (c *Catalog) ResolvePermissions(roleIDs []string) map[string]struct{} {
	out := make(map[string]struct{})
	if c == nil {
		return out
	}
	for i := range roleIDs {
		roleID := strings.TrimSpace(roleIDs[i])
		if roleID == "" {
			continue
		}
		set, ok := c.permissionsByRole[roleID]
		if !ok {
			continue
		}
		for permission := range set {
			out[permission] = struct{}{}
		}
	}
	return out
}

// ListRolePermissions returns all known roles with sorted permission ids.
func (c *Catalog) ListRolePermissions() []RolePermissions {
	if c == nil {
		return nil
	}
	roleIDs := make([]string, 0, len(c.permissionsByRole))
	for roleID := range c.permissionsByRole {
		roleIDs = append(roleIDs, roleID)
	}
	sort.Strings(roleIDs)
	out := make([]RolePermissions, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		set := c.permissionsByRole[roleID]
		perms := make([]string, 0, len(set))
		for p := range set {
			perms = append(perms, p)
		}
		sort.Strings(perms)
		out = append(out, RolePermissions{
			RoleID:      roleID,
			Permissions: perms,
		})
	}
	return out
}

func builtInRoles() map[string][]string {
	viewer := []string{
		permissions.StatusRead,
		permissions.RegistryRead,
		permissions.BundleRead,
		permissions.ControllerRead,
		permissions.TenantRead,
	}
	return map[string][]string{
		APIRoleAdmin:         {permissions.Wildcard},
		APIRoleViewer:        viewer,
		APIEdgeTokensIssue:   {permissions.EdgeTokenIssue},
		APIAccessTokensIssue: {permissions.APIAccessTokenIssue},
		APIContractsExport:   {permissions.ContractsExport},
	}
}

func normalizePermissionList(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for i := range in {
		p := strings.TrimSpace(in[i])
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func toPermissionSet(in []string) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for i := range in {
		out[in[i]] = struct{}{}
	}
	return out
}
