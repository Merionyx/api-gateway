package etcd

import (
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
)

func TestParseControllerInfoValue_ValidScope(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"ControllerID":"c1",
		"Tenant":"t1",
		"Environments":[
			{
				"Name":"dev",
				"Services":[
					{"Name":"svc1","Upstream":"u1","scope":"environment"},
					{"Name":"svc2","Upstream":"u2","scope":"controller_root"}
				]
			}
		]
	}`)

	parsed, err := parseControllerInfoValue(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(parsed.Environments) != 1 || len(parsed.Environments[0].Services) != 2 {
		t.Fatalf("unexpected parsed payload: %+v", parsed)
	}
	if parsed.Environments[0].Services[0].Scope != models.ServiceScopeEnvironment {
		t.Fatalf("scope got %q", parsed.Environments[0].Services[0].Scope)
	}
}

func TestParseControllerInfoValue_RejectsMissingScope(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"ControllerID":"c1",
		"Tenant":"t1",
		"Environments":[
			{
				"Name":"dev",
				"Services":[
					{"Name":"svc1","Upstream":"u1"}
				]
			}
		]
	}`)

	if _, err := parseControllerInfoValue(raw); err == nil {
		t.Fatal("expected error for missing scope")
	}
}

func TestParseControllerInfoValue_RejectsUnknownScope(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"ControllerID":"c1",
		"Tenant":"t1",
		"Environments":[
			{
				"Name":"dev",
				"Services":[
					{"Name":"svc1","Upstream":"u1","scope":"legacy"}
				]
			}
		]
	}`)

	if _, err := parseControllerInfoValue(raw); err == nil {
		t.Fatal("expected error for unknown scope")
	}
}
