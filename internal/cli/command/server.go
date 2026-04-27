package command

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	apiserverclient "github.com/merionyx/api-gateway/internal/cli/apiserver/client"
	"github.com/merionyx/api-gateway/internal/cli/apiserver/httpapi"
	"github.com/merionyx/api-gateway/internal/cli/outputfmt"
	"github.com/merionyx/api-gateway/internal/cli/style"

	"github.com/spf13/cobra"
)

// NewServerCommand builds `agwctl server ...` (API Server probes and metadata).
func NewServerCommand(resolveServer func() (string, error)) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "server",
		Short: "API Server: liveness, readiness, version, JWKS, dependency status",
	}
	cmd.PersistentFlags().StringVarP(&output, "output", "o", "table", "Output format: table, json, yaml (where structured output applies)")

	cmd.AddCommand(newServerPingCmd(resolveServer))
	cmd.AddCommand(newServerReadyCmd(resolveServer, &output))
	cmd.AddCommand(newServerVersionCmd(resolveServer, &output))
	keysCmd := &cobra.Command{
		Use:   "keys",
		Short: "Public keys / JWKS for JWT verification",
	}
	keysCmd.AddCommand(newServerKeysJwksCmd(resolveServer, &output))
	cmd.AddCommand(keysCmd)
	cmd.AddCommand(newServerStatusCmd(resolveServer, &output))
	return cmd
}

func newServerPingCmd(resolveServer func() (string, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "ping",
		Short: "Liveness: GET /health (process accepts HTTP)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			server, err := resolveServer()
			if err != nil {
				return err
			}
			httpClient, err := httpClientFromCmd(cmd)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			if err := httpapi.PingDefaultTimeout(ctx, httpClient, server); err != nil {
				return fmt.Errorf("ping %s: %w", server, err)
			}
			out := cmd.OutOrStdout()
			color := style.UseColorFor(out)
			_, _ = fmt.Fprintln(out)
			_, _ = fmt.Fprintf(out, "%s %s %s\n", style.S(color, style.Green, markOK), style.S(color, style.Green, "ok"), server)
			return nil
		},
	}
}

func newServerReadyCmd(resolveServer func() (string, error), output *string) *cobra.Command {
	return &cobra.Command{
		Use:   "ready",
		Short: "Readiness: GET /ready (etcd; optional Contract Syncer)",
		Long:  "Fetches readiness from the API Server. Exits with status 1 when HTTP 503 (not ready), after printing the response.\n",
		RunE: func(cmd *cobra.Command, _ []string) error {
			server, err := resolveServer()
			if err != nil {
				return err
			}
			httpClient, err := httpClientFromCmd(cmd)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			st, code, err := httpapi.Ready(ctx, httpClient, server)
			if err != nil {
				return err
			}
			of, err := outputfmt.Parse(*output)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			color := style.UseColorFor(out)
			switch of {
			case outputfmt.JSON, outputfmt.YAML:
				if err := outputfmt.Write(out, of, st); err != nil {
					return err
				}
			default:
				printReadinessTable(out, color, server, st, code)
			}
			if code != 200 {
				return fmt.Errorf("not ready (HTTP %d)", code)
			}
			return nil
		},
	}
}

func newServerVersionCmd(resolveServer func() (string, error), output *string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Server build metadata: GET /api/v1/version",
		Long:  "Shows git revision, build time, API schema version, and optional release tag for the API Server binary (not agwctl’s own version).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			server, err := resolveServer()
			if err != nil {
				return err
			}
			httpClient, err := httpClientFromCmd(cmd)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			v, err := httpapi.ServerVersion(ctx, httpClient, server)
			if err != nil {
				return err
			}
			of, err := outputfmt.Parse(*output)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			color := style.UseColorFor(out)
			switch of {
			case outputfmt.JSON, outputfmt.YAML:
				return outputfmt.Write(out, of, v)
			default:
				printVersionTable(out, color, server, v)
				return nil
			}
		},
	}
}

func newServerKeysJwksCmd(resolveServer func() (string, error), output *string) *cobra.Command {
	return &cobra.Command{
		Use:   "jwks",
		Short: "JWKS document: GET /.well-known/jwks.json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			server, err := resolveServer()
			if err != nil {
				return err
			}
			httpClient, err := httpClientFromCmd(cmd)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			j, err := httpapi.ServerJWKS(ctx, httpClient, server)
			if err != nil {
				return err
			}
			of, err := outputfmt.Parse(*output)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			color := style.UseColorFor(out)
			switch of {
			case outputfmt.JSON, outputfmt.YAML:
				return outputfmt.Write(out, of, j)
			default:
				printJWKSTable(out, color, server, j)
				return nil
			}
		},
	}
}

func newServerStatusCmd(resolveServer func() (string, error), output *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Dependency snapshot: GET /api/v1/status",
		Long:  "Rich status for operators (not a Kubernetes probe — use `server ready` for that).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			server, err := resolveServer()
			if err != nil {
				return err
			}
			httpClient, err := authorizedHTTPClientFromCmd(cmd, server)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			st, err := httpapi.ServerStatus(ctx, httpClient, server)
			if err != nil {
				return err
			}
			of, err := outputfmt.Parse(*output)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			color := style.UseColorFor(out)
			switch of {
			case outputfmt.JSON, outputfmt.YAML:
				return outputfmt.Write(out, of, st)
			default:
				printServerStatusTable(out, color, server, st)
				return nil
			}
		},
	}
}

func printReadinessTable(w io.Writer, color bool, serverURL string, st *apiserverclient.ReadinessStatus, httpCode int) {
	_, _ = fmt.Fprintln(w)
	title := style.S(color, style.Bold, "Readiness")
	codeHint := ""
	if httpCode == 503 {
		codeHint = " " + style.S(color, style.BoldRed, fmt.Sprintf("HTTP %d", httpCode)) + "\n"
	} else {
		codeHint = " " + style.S(color, style.Dim, fmt.Sprintf("HTTP %d", httpCode)) + "\n"
	}
	_, _ = fmt.Fprintf(w, "%s%s\n", title, codeHint)
	_, _ = fmt.Fprintf(w, "%s\n\n", style.S(color, style.Dim, serverURL+"/ready"))
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "%s\t%s\n", "status:", statusWordColor(color, st.Status))
	_, _ = fmt.Fprintf(tw, "%s\t%s\n", "etcd:", depWordColor(color, st.Etcd))
	_, _ = fmt.Fprintf(tw, "%s\t%s\n", "contract_syncer:", depWordColor(color, st.ContractSyncer))
	_ = tw.Flush()
	_, _ = fmt.Fprintln(w)
}

func printVersionTable(w io.Writer, color bool, serverURL string, v *apiserverclient.VersionResponse) {
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "%s\n", style.S(color, style.Bold, "API Server version\n"))
	_, _ = fmt.Fprintf(w, "%s\n", style.S(color, style.Dim, serverURL+"\n"))
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "%s\t%s\n", style.S(color, style.Dim, "git_revision:"), style.S(color, style.Cyan, v.GitRevision))
	_, _ = fmt.Fprintf(tw, "%s\t%s\n", style.S(color, style.Dim, "build_time:"), v.BuildTime)
	_, _ = fmt.Fprintf(tw, "%s\t%s\n", style.S(color, style.Dim, "api_schema_version:"), style.S(color, style.Cyan, v.ApiSchemaVersion))
	rel := "—"
	if v.Release != nil && strings.TrimSpace(*v.Release) != "" {
		rel = *v.Release
	}
	_, _ = fmt.Fprintf(tw, "%s\t%s\n", style.S(color, style.Dim, "release:"), rel)
	_ = tw.Flush()
	_, _ = fmt.Fprintln(w)
}

func printJWKSTable(w io.Writer, color bool, serverURL string, j *apiserverclient.Jwks) {
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "%s\n", style.S(color, style.Bold, "JWKS"))
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "%s\n", style.S(color, style.Dim, serverURL+"  "+"/.well-known/jwks.json"))
	_, _ = fmt.Fprintln(w)
	if len(j.Keys) == 0 {
		_, _ = fmt.Fprintf(w, "%s\n", style.S(color, style.Dim, "(no keys)"))
		_, _ = fmt.Fprintln(w)
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
		"KID",
		"KTY",
		"ALG",
		"USE",
		"NOTE")
	for i := range j.Keys {
		k := j.Keys[i]
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			strPtr(k.Kid),
			strPtr(k.Kty),
			strPtr(k.Alg),
			strPtr(k.Use),
			jwkNote(color, k))
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintln(w)
}

func jwkNote(color bool, k apiserverclient.Jwk) string {
	if k.Crv != nil && strings.TrimSpace(*k.Crv) != "" {
		return "crv=" + *k.Crv
	}
	if k.N != nil && len(*k.N) > 0 {
		return style.S(color, style.Dim, fmt.Sprintf("RSA n=%d B", len(*k.N)))
	}
	if k.X != nil && len(*k.X) > 0 {
		return style.S(color, style.Dim, fmt.Sprintf("coord=%d B", len(*k.X)))
	}
	return style.S(color, style.Dim, "—")
}

func strPtr(p *string) string {
	if p == nil {
		return "—"
	}
	s := strings.TrimSpace(*p)
	if s == "" {
		return "—"
	}
	return s
}

func printServerStatusTable(w io.Writer, color bool, serverURL string, st *apiserverclient.StatusResponse) {
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "%s\n\n", style.S(color, style.Bold, "Dependency status"))
	_, _ = fmt.Fprintf(w, "%s\n", style.S(color, style.Dim, serverURL+"/api/v1/status")+"\n")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "%s\t%s\n", "api_server:", depWordColor(color, st.ApiServer))
	_, _ = fmt.Fprintf(tw, "%s\t%s\n", "etcd:", depPtrColor(color, st.Etcd))
	_, _ = fmt.Fprintf(tw, "%s\t%s\n", "contract_syncer:", depPtrColor(color, st.ContractSyncer))
	_ = tw.Flush()
	_, _ = fmt.Fprintln(w)
}

func depPtrColor(color bool, p *string) string {
	if p == nil {
		return style.S(color, style.Dim, "—")
	}
	return depWordColor(color, *p)
}

func depWordColor(color bool, v string) string {
	vl := strings.ToLower(strings.TrimSpace(v))
	switch {
	case vl == "ok" || vl == "up":
		return style.S(color, style.Green, markOK+" "+v)
	case vl == "skipped" || vl == "unknown":
		return style.S(color, style.Dim, v)
	case strings.Contains(vl, "err"), vl == "error", vl == "down":
		return style.S(color, style.Red, markFail+" "+v)
	default:
		return style.S(color, style.Yellow, v)
	}
}

func statusWordColor(color bool, v string) string {
	vl := strings.ToLower(strings.TrimSpace(v))
	switch vl {
	case "ok":
		return style.S(color, style.Green, markOK+" "+v)
	case "not_ready":
		return style.S(color, style.BoldRed, markFail+" "+v)
	default:
		return style.S(color, style.Yellow, v)
	}
}
