package models

import "fmt"

const (
	ServiceScopeEnvironment    = "environment"
	ServiceScopeControllerRoot = "controller_root"
)

func IsValidServiceScope(scope string) bool {
	switch scope {
	case ServiceScopeEnvironment, ServiceScopeControllerRoot:
		return true
	default:
		return false
	}
}

// ValidateControllerServiceScopes enforces strict service scope contract for controller payloads.
func ValidateControllerServiceScopes(info ControllerInfo) error {
	return ValidateEnvironmentServiceScopes(info.Environments)
}

// ValidateEnvironmentServiceScopes enforces strict service scope contract for environment payloads.
func ValidateEnvironmentServiceScopes(envs []EnvironmentInfo) error {
	for envIdx := range envs {
		env := envs[envIdx]
		for svcIdx := range env.Services {
			svc := env.Services[svcIdx]
			if svc.Scope == "" {
				return fmt.Errorf("environment %q service %q: scope is required", env.Name, svc.Name)
			}
			if !IsValidServiceScope(svc.Scope) {
				return fmt.Errorf("environment %q service %q: unknown scope %q", env.Name, svc.Name, svc.Scope)
			}
		}
	}
	return nil
}
