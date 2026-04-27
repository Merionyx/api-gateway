package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/merionyx/api-gateway/internal/cli/apiserver/httpapi"
	"github.com/merionyx/api-gateway/internal/cli/bundleopt"
	"github.com/merionyx/api-gateway/internal/cli/describefmt"
	"github.com/merionyx/api-gateway/internal/cli/outputfmt"
	"github.com/merionyx/api-gateway/internal/cli/resource"
	"github.com/merionyx/api-gateway/internal/cli/style"

	"github.com/spf13/cobra"
)

// NewDescribeCommand builds `agwctl describe RESOURCE [NAME]`.
func NewDescribeCommand(resolveServer func() (string, error)) *cobra.Command {
	var (
		output      string
		tenant      string
		bundleKey   string
		repo        string
		ref         string
		bundlePath  string
		environment string
		envAlias    string
	)

	cmd := &cobra.Command{
		Use:   "describe RESOURCE [NAME]",
		Short: "Show details for one registry resource",
		Long: fmt.Sprintf(`Describe loads a single item: direct GET when the API supports it, otherwise
exact match by walking paginated list responses (same page size as list: %d).

Canonical resource names (plural): %s

Controllers: NAME is controller_id.
Tenants: NAME is the tenant string (from the aggregated list).
bundle-keys: NAME is the full bundle key, unless you pass --bundle-key or --repo/--ref/--path instead of NAME.
Environments: requires --tenant; NAME is the environment name.
Bundles: requires --tenant; NAME is the bundle logical name (optional --environment / --env to scope).
Contract document: NAME is the contract name; bundle via --bundle-key or --repo/--ref (optional --path).

Aliases match list (e.g. ctrl, env, bk, cn).`,
			httpapi.MaxPageSize, resource.CanonicalNames()),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, ok := resource.Resolve(args[0])
			if !ok {
				return fmt.Errorf("unknown resource %q; supported: %s", args[0], resource.CanonicalNames())
			}
			if entry.RequiresTenant && strings.TrimSpace(tenant) == "" {
				return fmt.Errorf("--tenant is required for resource %q", entry.Canonical)
			}

			nameArg := ""
			if len(args) > 1 {
				nameArg = strings.TrimSpace(args[1])
			}
			if entry.Kind != resource.BundleKeys && nameArg == "" {
				return fmt.Errorf("NAME is required for resource %q", entry.Canonical)
			}
			if entry.Kind == resource.ContractNames && nameArg == "" {
				return fmt.Errorf("NAME (contract name) is required for contract-names")
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
				return fmt.Errorf("--environment / --env applies only to describe bundles")
			}

			if entry.Kind != resource.ContractNames && entry.Kind != resource.BundleKeys {
				if bundleKey != "" || repo != "" || ref != "" || bundlePath != "" {
					return fmt.Errorf("--bundle-key / --repo / --ref / --path apply only to contract-names and bundle-keys")
				}
			}

			var resolvedBundleKey string
			switch entry.Kind {
			case resource.ContractNames:
				var err error
				resolvedBundleKey, err = bundleopt.ResolveBundleKey(bundleKey, repo, ref, bundlePath)
				if err != nil {
					return err
				}
			case resource.BundleKeys:
				var err error
				resolvedBundleKey, err = bundleopt.ResolveBundleKeyOrName(bundleKey, repo, ref, bundlePath, nameArg)
				if err != nil {
					return err
				}
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

			ctx := context.Background()
			if cctx := cmd.Context(); cctx != nil {
				ctx = cctx
			}

			tTrim := strings.TrimSpace(tenant)

			v, err := runDescribe(ctx, httpClient, server, entry.Kind, nameArg, tTrim, resolvedBundleKey, envFilter)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			color := style.UseColorFor(out)
			_, _ = fmt.Fprintln(out)
			return writeDescribe(out, color, of, v)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table (tree), json, yaml")
	cmd.Flags().StringVarP(&tenant, "tenant", "t", "", "Tenant name (required for environments and bundles; not used for controllers, tenants, bundle-keys, contract-names)")
	cmd.Flags().StringVarP(&environment, "environment", "e", "", "When describing bundles: search only this environment (requires --tenant)")
	cmd.Flags().StringVar(&envAlias, "env", "", "Alias for --environment")
	cmd.Flags().StringVar(&bundleKey, "bundle-key", "", "Full bundle key (for contract-names / bundle-keys; mutually exclusive with --repo/--ref)")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository id (with --ref) instead of --bundle-key")
	cmd.Flags().StringVar(&ref, "ref", "", "Git ref (with --repo) instead of --bundle-key")
	cmd.Flags().StringVar(&bundlePath, "path", "", "Logical bundle root path (optional with --repo/--ref)")
	return cmd
}

func runDescribe(ctx context.Context, hc *http.Client, server string, kind resource.Kind, name, tenant, bundleKey, environment string) (any, error) {
	switch kind {
	case resource.Controllers:
		return httpapi.GetController(ctx, hc, server, name)
	case resource.Tenants:
		t, err := httpapi.FindTenant(ctx, hc, server, name)
		if err != nil {
			return nil, err
		}
		return map[string]any{"tenant": t}, nil
	case resource.Environments:
		return httpapi.FindEnvironment(ctx, hc, server, tenant, name)
	case resource.Bundles:
		return httpapi.FindBundle(ctx, hc, server, tenant, environment, name)
	case resource.BundleKeys:
		k, err := httpapi.FindBundleKey(ctx, hc, server, bundleKey)
		if err != nil {
			return nil, err
		}
		return map[string]any{"bundle_key": k}, nil
	case resource.ContractNames:
		return httpapi.GetContractDocument(ctx, hc, server, bundleKey, name)
	default:
		return nil, fmt.Errorf("unsupported resource kind %q", kind)
	}
}

func writeDescribe(w io.Writer, color bool, of outputfmt.Format, v any) error {
	switch of {
	case outputfmt.JSON, outputfmt.YAML:
		return outputfmt.Write(w, of, v)
	default:
		tree, err := anyAsDescribeTree(v)
		if err != nil {
			return err
		}
		return describefmt.Write(w, tree, color)
	}
}

func anyAsDescribeTree(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	return describefmt.Normalize(raw), nil
}
