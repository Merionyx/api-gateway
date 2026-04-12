package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	apiserverclient "github.com/merionyx/api-gateway/internal/cli/apiserver/client"
)

func errControllerGet(resp *apiserverclient.GetControllerResponse) error {
	if resp.ApplicationproblemJSON404 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON404))
	}
	if resp.ApplicationproblemJSON500 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON500))
	}
	return fmt.Errorf("api: HTTP %d: %s", resp.StatusCode(), trimBody(resp.Body))
}

func errContractGet(resp *apiserverclient.GetContractInBundleResponse) error {
	if resp.ApplicationproblemJSON404 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON404))
	}
	if resp.ApplicationproblemJSON500 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON500))
	}
	return fmt.Errorf("api: HTTP %d: %s", resp.StatusCode(), trimBody(resp.Body))
}

// GetController calls GET /api/v1/controllers/{controller_id}.
func GetController(ctx context.Context, httpClient *http.Client, serverURL, controllerID string) (*apiserverclient.Controller, error) {
	c, err := newClientWithResponses(serverURL, httpClient)
	if err != nil {
		return nil, err
	}
	id := apiserverclient.ControllerId(strings.TrimSpace(controllerID))
	resp, err := c.GetControllerWithResponse(ctx, id, &apiserverclient.GetControllerParams{})
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errControllerGet(resp)
}

// GetContractDocument calls GET /api/v1/bundles/contracts/{contract_name} (bundle_key query).
func GetContractDocument(ctx context.Context, httpClient *http.Client, serverURL, bundleKey, contractName string) (apiserverclient.ContractDocument, error) {
	c, err := newClientWithResponses(serverURL, httpClient)
	if err != nil {
		return nil, err
	}
	bkq := apiserverclient.BundleKeyQuery(bundleKey)
	cn := apiserverclient.ContractName(contractName)
	params := &apiserverclient.GetContractInBundleParams{BundleKey: &bkq}
	resp, err := c.GetContractInBundleWithResponse(ctx, cn, params)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return nil, errContractGet(resp)
}

// FindTenant walks paginated GET /api/v1/tenants until name matches (exact).
func FindTenant(ctx context.Context, httpClient *http.Client, serverURL, name string) (string, error) {
	want := strings.TrimSpace(name)
	if want == "" {
		return "", fmt.Errorf("tenant name is empty")
	}
	var walk *string
	for {
		resp, err := ListTenants(ctx, httpClient, serverURL, walk)
		if err != nil {
			return "", err
		}
		for _, t := range resp.Items {
			if t == want {
				return t, nil
			}
		}
		if !resp.HasMore || resp.NextCursor == nil || strings.TrimSpace(*resp.NextCursor) == "" {
			break
		}
		walk = resp.NextCursor
	}
	return "", fmt.Errorf("tenant %q not found", want)
}

// FindEnvironment walks GET .../tenants/{tenant}/environments until environment name matches.
func FindEnvironment(ctx context.Context, httpClient *http.Client, serverURL, tenant, envName string) (*apiserverclient.Environment, error) {
	tn := strings.TrimSpace(tenant)
	want := strings.TrimSpace(envName)
	if tn == "" {
		return nil, fmt.Errorf("tenant is required")
	}
	if want == "" {
		return nil, fmt.Errorf("environment name is empty")
	}
	var walk *string
	for {
		resp, err := ListEnvironmentsByTenant(ctx, httpClient, serverURL, tn, walk)
		if err != nil {
			return nil, err
		}
		for i := range resp.Items {
			e := &resp.Items[i]
			if e.Name == want {
				return e, nil
			}
		}
		if !resp.HasMore || resp.NextCursor == nil || strings.TrimSpace(*resp.NextCursor) == "" {
			break
		}
		walk = resp.NextCursor
	}
	return nil, fmt.Errorf("environment %q not found for tenant %q", want, tn)
}

func bundleNameMatch(b apiserverclient.Bundle, want string) bool {
	if b.Name == nil {
		return false
	}
	return *b.Name == want
}

// FindBundle resolves a static bundle descriptor by name for a tenant.
// If environment is non-empty, searches only that environment’s bundles; otherwise walks the flattened tenant bundle list.
func FindBundle(ctx context.Context, httpClient *http.Client, serverURL, tenant, environment, bundleName string) (*apiserverclient.Bundle, error) {
	tn := strings.TrimSpace(tenant)
	want := strings.TrimSpace(bundleName)
	env := strings.TrimSpace(environment)
	if tn == "" {
		return nil, fmt.Errorf("tenant is required for bundles")
	}
	if want == "" {
		return nil, fmt.Errorf("bundle name is empty")
	}
	if env != "" {
		return findBundleInEnvironment(ctx, httpClient, serverURL, tn, env, want)
	}
	var walk *string
	for {
		resp, err := ListBundlesByTenant(ctx, httpClient, serverURL, tn, walk)
		if err != nil {
			return nil, err
		}
		for i := range resp.Items {
			b := resp.Items[i]
			if bundleNameMatch(b, want) {
				bb := b
				return &bb, nil
			}
		}
		if !resp.HasMore || resp.NextCursor == nil || strings.TrimSpace(*resp.NextCursor) == "" {
			break
		}
		walk = resp.NextCursor
	}
	return nil, fmt.Errorf("bundle %q not found for tenant %q", want, tn)
}

func findBundleInEnvironment(ctx context.Context, httpClient *http.Client, serverURL, tenant, envName, bundleWant string) (*apiserverclient.Bundle, error) {
	var walk *string
	for {
		resp, err := ListEnvironmentsByTenant(ctx, httpClient, serverURL, tenant, walk)
		if err != nil {
			return nil, err
		}
		for i := range resp.Items {
			e := &resp.Items[i]
			if e.Name != envName {
				continue
			}
			if e.Bundles == nil {
				return nil, fmt.Errorf("bundle %q not found in environment %q for tenant %q", bundleWant, envName, tenant)
			}
			for j := range *e.Bundles {
				b := (*e.Bundles)[j]
				if bundleNameMatch(b, bundleWant) {
					bb := b
					return &bb, nil
				}
			}
			return nil, fmt.Errorf("bundle %q not found in environment %q for tenant %q", bundleWant, envName, tenant)
		}
		if !resp.HasMore || resp.NextCursor == nil || strings.TrimSpace(*resp.NextCursor) == "" {
			break
		}
		walk = resp.NextCursor
	}
	return nil, fmt.Errorf("environment %q not found for tenant %q", envName, tenant)
}

// FindBundleKey walks paginated GET /api/v1/bundles until the bundle key string matches (exact).
func FindBundleKey(ctx context.Context, httpClient *http.Client, serverURL, key string) (string, error) {
	want := strings.TrimSpace(key)
	if want == "" {
		return "", fmt.Errorf("bundle key is empty")
	}
	var walk *string
	for {
		resp, err := ListBundleKeys(ctx, httpClient, serverURL, walk)
		if err != nil {
			return "", err
		}
		for _, b := range resp.Items {
			if b.Key != nil && *b.Key == want {
				return want, nil
			}
		}
		if !resp.HasMore || resp.NextCursor == nil || strings.TrimSpace(*resp.NextCursor) == "" {
			break
		}
		walk = resp.NextCursor
	}
	return "", fmt.Errorf("bundle key %q not found", want)
}
