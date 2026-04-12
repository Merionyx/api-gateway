package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	apiserverclient "github.com/merionyx/api-gateway/internal/cli/apiserver/client"
)

// ListBundles lists static bundle descriptors for a tenant (GET .../tenants/{tenant}/bundles).
// If environment is non-empty, resolves bundles for that environment name by scanning
// GET .../tenants/{tenant}/environments until a matching environment is found (MaxPageSize per request).
func ListBundles(ctx context.Context, httpClient *http.Client, serverURL, tenant, environment string, cursor *string) (*apiserverclient.BundleRefListResponse, error) {
	tenant = strings.TrimSpace(tenant)
	if tenant == "" {
		return nil, fmt.Errorf("tenant is required for bundles")
	}
	env := strings.TrimSpace(environment)
	if env == "" {
		return ListBundlesByTenant(ctx, httpClient, serverURL, tenant, cursor)
	}
	return bundlesInEnvironment(ctx, httpClient, serverURL, tenant, env)
}

func bundlesInEnvironment(ctx context.Context, httpClient *http.Client, serverURL, tenant, envName string) (*apiserverclient.BundleRefListResponse, error) {
	var walk *string
	for {
		resp, err := ListEnvironmentsByTenant(ctx, httpClient, serverURL, tenant, walk)
		if err != nil {
			return nil, err
		}
		for i := range resp.Items {
			e := &resp.Items[i]
			if e.Name == envName {
				var items []apiserverclient.Bundle
				if e.Bundles != nil {
					items = *e.Bundles
				}
				return &apiserverclient.BundleRefListResponse{
					HasMore:    false,
					Items:      items,
					NextCursor: nil,
				}, nil
			}
		}
		if !resp.HasMore || resp.NextCursor == nil || strings.TrimSpace(*resp.NextCursor) == "" {
			break
		}
		walk = resp.NextCursor
	}
	return nil, fmt.Errorf("environment %q not found for tenant %q", envName, tenant)
}
