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
