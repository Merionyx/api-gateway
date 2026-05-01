package roles

import (
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/auth/permissions"
)

func TestCatalogBuiltIns(t *testing.T) {
	t.Parallel()
	c, err := NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	admin := c.ResolvePermissions([]string{APIRoleAdmin})
	if _, ok := admin[permissions.Wildcard]; !ok {
		t.Fatalf("admin permissions %+v", admin)
	}
	viewer := c.ResolvePermissions([]string{APIRoleViewer})
	if _, ok := viewer[permissions.RegistryRead]; !ok {
		t.Fatalf("viewer permissions %+v", viewer)
	}
}

func TestCatalogConfiguredRole(t *testing.T) {
	t.Parallel()
	c, err := NewCatalog([]ConfiguredRole{{
		ID:          "api:role:token-generator",
		Permissions: []string{"api.token.edge.issue"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := c.ResolvePermissions([]string{"api:role:token-generator"})
	if _, ok := got["api.token.edge.issue"]; !ok {
		t.Fatalf("permissions %+v", got)
	}
}

func TestCatalogListRolePermissions_sorted(t *testing.T) {
	t.Parallel()

	c, err := NewCatalog([]ConfiguredRole{
		{ID: "zzz", Permissions: []string{"p3", "p1", "p2"}},
		{ID: "aaa", Permissions: []string{"p2", "p1"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	rows := c.ListRolePermissions()
	if len(rows) == 0 {
		t.Fatal("empty role list")
	}
	for i := 1; i < len(rows); i++ {
		if rows[i-1].RoleID > rows[i].RoleID {
			t.Fatalf("roles not sorted: %q then %q", rows[i-1].RoleID, rows[i].RoleID)
		}
	}
	for i := range rows {
		for j := 1; j < len(rows[i].Permissions); j++ {
			if rows[i].Permissions[j-1] >= rows[i].Permissions[j] {
				t.Fatalf("permissions for %q are not sorted: %v", rows[i].RoleID, rows[i].Permissions)
			}
		}
	}
}
