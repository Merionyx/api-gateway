package command

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/merionyx/api-gateway/internal/cli/apiserver/httpapi"
	"github.com/merionyx/api-gateway/internal/cli/bundleopt"
	"github.com/merionyx/api-gateway/internal/cli/outputfmt"
	"github.com/merionyx/api-gateway/internal/cli/resource"
	"github.com/merionyx/api-gateway/internal/cli/style"

	"github.com/spf13/cobra"
)

// NewListCommand builds `agwctl list RESOURCE` (kubectl-style; supports -o json|yaml).
func NewListCommand(resolveServer func() (string, error)) *cobra.Command {
	var (
		output      string
		cursor      string
		tenant      string
		bundleKey   string
		repo        string
		ref         string
		bundlePath  string
		environment string
		envAlias    string
	)

	cmd := &cobra.Command{
		Use:   "list RESOURCE",
		Short: "List registry resources from the API Server",
		Long: fmt.Sprintf(`List reads collection endpoints (cursor pagination where supported).
Each request asks for the maximum page size allowed by the API Server (%d items per page).

Canonical resource names (plural): %s

Use --tenant (-t) where required; for controllers it is optional (scopes to that tenant).
For bundles, --tenant is required; optional --environment (-e) / --env filters bundles to one environment.

For contract-names, identify the bundle with --bundle-key or with --repo and --ref (optional --path).

Aliases are accepted (e.g. controllers, ctrl, cnt; bundles, tenant-bundles; environments, env).`,
			httpapi.MaxPageSize, resource.CanonicalNames()),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, ok := resource.Resolve(args[0])
			if !ok {
				return fmt.Errorf("unknown resource %q; supported: %s", args[0], resource.CanonicalNames())
			}
			if entry.RequiresTenant && strings.TrimSpace(tenant) == "" {
				return fmt.Errorf("--tenant is required for resource %q", entry.Canonical)
			}
			var resolvedBundleKey string
			if entry.Kind == resource.ContractNames {
				var err error
				resolvedBundleKey, err = bundleopt.ResolveBundleKey(bundleKey, repo, ref, bundlePath)
				if err != nil {
					return err
				}
			} else if bundleKey != "" || repo != "" || ref != "" || bundlePath != "" {
				return fmt.Errorf("--bundle-key / --repo / --ref / --path apply only to list contract-names")
			}

			e1 := strings.TrimSpace(environment)
			e2 := strings.TrimSpace(envAlias)
			var envFilter string
			switch {
			case e1 != "" && e2 != "" && e1 != e2:
				return fmt.Errorf("--environment and --env disagree (%q vs %q)", e1, e2)
			case e1 != "":
				envFilter = e1
			default:
				envFilter = e2
			}
			if envFilter != "" && entry.Kind != resource.Bundles {
				return fmt.Errorf("--environment / --env applies only to list bundles")
			}

			of, err := outputfmt.Parse(output)
			if err != nil {
				return err
			}

			server, err := resolveServer()
			if err != nil {
				return err
			}
			httpClient, err := authorizedHTTPClientFromCmd(cmd, server)
			if err != nil {
				return err
			}

			var cursorPtr *string
			if c := strings.TrimSpace(cursor); cmd.Flags().Changed("cursor") && c != "" {
				cursorPtr = &c
			}

			ctx := context.Background()
			if cctx := cmd.Context(); cctx != nil {
				ctx = cctx
			}

			tTrim := strings.TrimSpace(tenant)

			v, err := runList(ctx, httpClient, server, entry.Kind, cursorPtr, tTrim, resolvedBundleKey, envFilter)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			color := style.UseColorFor(out)
			_, _ = fmt.Fprintln(out)

			switch of {
			case outputfmt.JSON, outputfmt.YAML:
				if err := outputfmt.Write(out, of, v); err != nil {
					return err
				}
			default:
				printListTable(out, color, entry.Kind, v)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table, json, yaml")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor from a previous response (not used with list bundles --environment/--env)")
	cmd.Flags().StringVarP(&tenant, "tenant", "t", "", "Tenant name (required for environments and bundles; optional for controllers)")
	cmd.Flags().StringVarP(&environment, "environment", "e", "", "When listing bundles: keep only bundles for this environment name (requires --tenant)")
	cmd.Flags().StringVar(&envAlias, "env", "", "Alias for --environment")
	cmd.Flags().StringVar(&bundleKey, "bundle-key", "", "Full bundle key (for contract-names; mutually exclusive with --repo/--ref)")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository id (for contract-names; use with --ref)")
	cmd.Flags().StringVar(&ref, "ref", "", "Git ref (for contract-names; use with --repo)")
	cmd.Flags().StringVar(&bundlePath, "path", "", "Logical bundle root path inside the repo (optional; for contract-names with --repo/--ref)")
	return cmd
}

func runList(ctx context.Context, hc *http.Client, server string, kind resource.Kind, cursor *string, tenant, bundleKey, environment string) (any, error) {
	switch kind {
	case resource.Controllers:
		if tenant != "" {
			return httpapi.ListControllersByTenant(ctx, hc, server, tenant, cursor)
		}
		return httpapi.ListControllers(ctx, hc, server, cursor)
	case resource.Tenants:
		return httpapi.ListTenants(ctx, hc, server, cursor)
	case resource.Environments:
		return httpapi.ListEnvironmentsByTenant(ctx, hc, server, tenant, cursor)
	case resource.Bundles:
		return httpapi.ListBundles(ctx, hc, server, tenant, environment, cursor)
	case resource.BundleKeys:
		return httpapi.ListBundleKeys(ctx, hc, server, cursor)
	case resource.ContractNames:
		return httpapi.ListContractNamesInBundle(ctx, hc, server, bundleKey, cursor)
	default:
		return nil, fmt.Errorf("unsupported resource kind %q", kind)
	}
}
