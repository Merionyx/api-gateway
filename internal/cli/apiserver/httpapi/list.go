package httpapi

import (
	"context"
	"fmt"
	"net/http"

	apiserverclient "github.com/merionyx/api-gateway/internal/cli/apiserver/client"
)

// MaxPageSize is the maximum value for the list "limit" query parameter (OpenAPI maximum for API Server).
const MaxPageSize = 500

func maxListLimit() *apiserverclient.Limit {
	x := apiserverclient.Limit(MaxPageSize)
	return &x
}

func strCursor(p *string) *apiserverclient.Cursor {
	if p == nil || *p == "" {
		return nil
	}
	x := apiserverclient.Cursor(*p)
	return &x
}

func errControllersList(resp *apiserverclient.ListControllersResponse) error {
	if resp.ApplicationproblemJSON400 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON400))
	}
	if resp.ApplicationproblemJSON500 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON500))
	}
	return fmt.Errorf("api: HTTP %d: %s", resp.StatusCode(), trimBody(resp.Body))
}

func errTenantsList(resp *apiserverclient.ListTenantsResponse) error {
	if resp.ApplicationproblemJSON400 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON400))
	}
	if resp.ApplicationproblemJSON500 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON500))
	}
	return fmt.Errorf("api: HTTP %d: %s", resp.StatusCode(), trimBody(resp.Body))
}

func errControllersByTenant(resp *apiserverclient.ListControllersByTenantResponse) error {
	if resp.ApplicationproblemJSON400 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON400))
	}
	if resp.ApplicationproblemJSON500 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON500))
	}
	return fmt.Errorf("api: HTTP %d: %s", resp.StatusCode(), trimBody(resp.Body))
}

func errEnvironmentsByTenant(resp *apiserverclient.ListEnvironmentsByTenantResponse) error {
	if resp.ApplicationproblemJSON400 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON400))
	}
	if resp.ApplicationproblemJSON500 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON500))
	}
	return fmt.Errorf("api: HTTP %d: %s", resp.StatusCode(), trimBody(resp.Body))
}

func errBundlesByTenant(resp *apiserverclient.ListBundlesByTenantResponse) error {
	if resp.ApplicationproblemJSON400 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON400))
	}
	if resp.ApplicationproblemJSON500 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON500))
	}
	return fmt.Errorf("api: HTTP %d: %s", resp.StatusCode(), trimBody(resp.Body))
}

func errBundleKeys(resp *apiserverclient.ListBundleKeysResponse) error {
	if resp.ApplicationproblemJSON400 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON400))
	}
	if resp.ApplicationproblemJSON500 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON500))
	}
	return fmt.Errorf("api: HTTP %d: %s", resp.StatusCode(), trimBody(resp.Body))
}

func errContractNames(resp *apiserverclient.ListContractsInBundleResponse) error {
	if resp.ApplicationproblemJSON400 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON400))
	}
	if resp.ApplicationproblemJSON404 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON404))
	}
	if resp.ApplicationproblemJSON500 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON500))
	}
	return fmt.Errorf("api: HTTP %d: %s", resp.StatusCode(), trimBody(resp.Body))
}

// ListControllers calls GET /api/v1/controllers.
func ListControllers(ctx context.Context, httpClient *http.Client, serverURL string, cursor *string) (*apiserverclient.ControllerListResponse, error) {
	c, err := newClientWithResponses(serverURL, httpClient)
	if err != nil {
		return nil, err
	}
	params := &apiserverclient.ListControllersParams{
		Limit:  maxListLimit(),
		Cursor: strCursor(cursor),
	}
	resp, err := c.ListControllersWithResponse(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errControllersList(resp)
}

// ListTenants calls GET /api/v1/tenants.
func ListTenants(ctx context.Context, httpClient *http.Client, serverURL string, cursor *string) (*apiserverclient.TenantListResponse, error) {
	c, err := newClientWithResponses(serverURL, httpClient)
	if err != nil {
		return nil, err
	}
	params := &apiserverclient.ListTenantsParams{
		Limit:  maxListLimit(),
		Cursor: strCursor(cursor),
	}
	resp, err := c.ListTenantsWithResponse(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errTenantsList(resp)
}

// ListControllersByTenant calls GET /api/v1/tenants/{tenant}/controllers.
func ListControllersByTenant(ctx context.Context, httpClient *http.Client, serverURL, tenant string, cursor *string) (*apiserverclient.ControllerListResponse, error) {
	c, err := newClientWithResponses(serverURL, httpClient)
	if err != nil {
		return nil, err
	}
	t := apiserverclient.Tenant(tenant)
	params := &apiserverclient.ListControllersByTenantParams{
		Limit:  maxListLimit(),
		Cursor: strCursor(cursor),
	}
	resp, err := c.ListControllersByTenantWithResponse(ctx, t, params)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errControllersByTenant(resp)
}

// ListEnvironmentsByTenant calls GET /api/v1/tenants/{tenant}/environments.
func ListEnvironmentsByTenant(ctx context.Context, httpClient *http.Client, serverURL, tenant string, cursor *string) (*apiserverclient.EnvironmentListResponse, error) {
	c, err := newClientWithResponses(serverURL, httpClient)
	if err != nil {
		return nil, err
	}
	t := apiserverclient.Tenant(tenant)
	params := &apiserverclient.ListEnvironmentsByTenantParams{
		Limit:  maxListLimit(),
		Cursor: strCursor(cursor),
	}
	resp, err := c.ListEnvironmentsByTenantWithResponse(ctx, t, params)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errEnvironmentsByTenant(resp)
}

// ListBundlesByTenant calls GET /api/v1/tenants/{tenant}/bundles.
func ListBundlesByTenant(ctx context.Context, httpClient *http.Client, serverURL, tenant string, cursor *string) (*apiserverclient.BundleRefListResponse, error) {
	c, err := newClientWithResponses(serverURL, httpClient)
	if err != nil {
		return nil, err
	}
	t := apiserverclient.Tenant(tenant)
	params := &apiserverclient.ListBundlesByTenantParams{
		Limit:  maxListLimit(),
		Cursor: strCursor(cursor),
	}
	resp, err := c.ListBundlesByTenantWithResponse(ctx, t, params)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errBundlesByTenant(resp)
}

// ListBundleKeys calls GET /api/v1/bundles.
func ListBundleKeys(ctx context.Context, httpClient *http.Client, serverURL string, cursor *string) (*apiserverclient.BundleKeyListResponse, error) {
	c, err := newClientWithResponses(serverURL, httpClient)
	if err != nil {
		return nil, err
	}
	params := &apiserverclient.ListBundleKeysParams{
		Limit:  maxListLimit(),
		Cursor: strCursor(cursor),
	}
	resp, err := c.ListBundleKeysWithResponse(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errBundleKeys(resp)
}

// ListContractNamesInBundle calls GET /api/v1/bundles/{bundle_key}/contracts.
func ListContractNamesInBundle(ctx context.Context, httpClient *http.Client, serverURL, bundleKey string, cursor *string) (*apiserverclient.ContractNameListResponse, error) {
	c, err := newClientWithResponses(serverURL, httpClient)
	if err != nil {
		return nil, err
	}
	bk := apiserverclient.BundleKey(bundleKey)
	params := &apiserverclient.ListContractsInBundleParams{
		Limit:  maxListLimit(),
		Cursor: strCursor(cursor),
	}
	resp, err := c.ListContractsInBundleWithResponse(ctx, bk, params)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, errContractNames(resp)
}
