package models

import "testing"

func TestValidateControllerServiceScopes(t *testing.T) {
	t.Parallel()

	valid := ControllerInfo{
		Environments: []EnvironmentInfo{
			{
				Name: "dev",
				Services: []ServiceInfo{
					{Name: "svc-env", Scope: ServiceScopeEnvironment},
					{Name: "svc-root", Scope: ServiceScopeControllerRoot},
				},
			},
		},
	}
	if err := ValidateControllerServiceScopes(valid); err != nil {
		t.Fatalf("expected valid payload, got error: %v", err)
	}

	missingScope := ControllerInfo{
		Environments: []EnvironmentInfo{
			{
				Name: "dev",
				Services: []ServiceInfo{
					{Name: "svc-env", Scope: ""},
				},
			},
		},
	}
	if err := ValidateControllerServiceScopes(missingScope); err == nil {
		t.Fatal("expected error for missing scope")
	}

	unknownScope := ControllerInfo{
		Environments: []EnvironmentInfo{
			{
				Name: "dev",
				Services: []ServiceInfo{
					{Name: "svc-env", Scope: "unknown"},
				},
			},
		},
	}
	if err := ValidateControllerServiceScopes(unknownScope); err == nil {
		t.Fatal("expected error for unknown scope")
	}
}
