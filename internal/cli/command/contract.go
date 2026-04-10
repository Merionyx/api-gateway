// Package command registers cobra subcommands for agwctl.
package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/merionyx/api-gateway/internal/cli/apiclient"
	"github.com/merionyx/api-gateway/internal/cli/contractdiff"
	"github.com/merionyx/api-gateway/internal/cli/contractfmt"
	"github.com/merionyx/api-gateway/internal/cli/outfiles"
	"github.com/merionyx/api-gateway/internal/cli/style"
	"github.com/merionyx/api-gateway/internal/cli/validate"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewContractCommand builds `agwctl contract ...`. resolveServer returns the API Server base URL.
func NewContractCommand(resolveServer func() (string, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contract",
		Short: "Contract schema files",
	}
	cmd.AddCommand(
		newExportCmd(resolveServer),
		newExportBatchCmd(resolveServer),
		newDiffCmd(resolveServer),
		newDiffBatchCmd(resolveServer),
		newFmtCmd(),
	)
	return cmd
}

func newFmtCmd() *cobra.Command {
	var write bool
	var format string
	c := &cobra.Command{
		Use:   "fmt PATH",
		Short: "Canonicalize contract YAML/JSON (OpenAPI 3.x via kin-openapi, x-api-gateway v1 with fixed field order)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			fi, err := os.Stat(filepath.Clean(target))
			if err != nil {
				return err
			}
			if fi.IsDir() && !write {
				return fmt.Errorf("formatting a directory requires --write")
			}

			paths, err := validate.CollectFiles(target)
			if err != nil {
				return err
			}
			if len(paths) == 0 {
				return fmt.Errorf("no .yaml/.yml/.json files under %s", target)
			}

			out := cmd.OutOrStdout()
			logErr := func(format string, a ...any) { fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", a...) }
			var nwrite, nfail int
			for _, p := range paths {
				data, err := os.ReadFile(p)
				if err != nil {
					logErr("%s: read: %v", p, err)
					nfail++
					continue
				}
				ext := filepath.Ext(p)
				formatted, err := contractfmt.FormatBytes(data, ext, format)
				if err != nil {
					logErr("%s: %v", p, err)
					nfail++
					continue
				}
				if write {
					if err := os.WriteFile(p, formatted, 0o644); err != nil {
						logErr("%s: write: %v", p, err)
						nfail++
						continue
					}
					nwrite++
					continue
				}
				if len(paths) > 1 {
					fmt.Fprintf(out, "# %s\n", p)
				}
				if _, err := out.Write(formatted); err != nil {
					return err
				}
				if len(paths) > 1 {
					fmt.Fprintln(out)
				}
			}
			if write && fi.IsDir() {
				out := cmd.OutOrStdout()
				fmt.Fprintln(out)
				fmt.Fprintf(out, "%s %s %d file(s) %s %s\n", style.S(style.UseColorFor(out), style.Green, markOK), style.S(style.UseColorFor(out), style.Dim, "formated"), nwrite, style.S(style.UseColorFor(out), style.Dim, "under"), target)
			}
			if nfail > 0 {
				return fmt.Errorf("format failed for %d file(s)", nfail)
			}
			return nil
		},
	}
	c.Flags().BoolVarP(&write, "write", "w", false, "write result to source file(s) instead of stdout")
	c.Flags().StringVar(&format, "format", "yaml", "output: yaml or json")
	return c
}

func newExportCmd(resolveServer func() (string, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "export",
		Short: "Export contracts from git (via API Server -> Contract Syncer)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			server, err := resolveServer()
			if err != nil {
				return err
			}
			repo, _ := cmd.Flags().GetString("repo")
			ref, _ := cmd.Flags().GetString("ref")
			path, _ := cmd.Flags().GetString("path")
			contract, _ := cmd.Flags().GetString("contract")
			out, _ := cmd.Flags().GetString("out")
			format, _ := cmd.Flags().GetString("format")

			httpClient, err := httpClientFromCmd(cmd)
			if err != nil {
				return err
			}
			files, err := apiclient.ExportContracts(context.Background(), httpClient, server, apiclient.ExportRequest{
				Repository:   repo,
				Ref:          ref,
				Path:         path,
				ContractName: contract,
			})
			if err != nil {
				return err
			}
			return outfiles.WriteExported(files, out, format)
		},
	}
	c.Flags().String("repo", "", "repository name (as in contract-syncer config)")
	c.Flags().String("ref", "", "git ref (e.g. heads/main, remotes/origin/master)")
	c.Flags().String("path", "", "path inside repository (omit for entire repo)")
	c.Flags().String("contract", "", "single contract name (omit for all)")
	c.Flags().String("out", "", "output directory")
	c.Flags().String("format", "", "optional: yaml or json (converts on CLI; default keeps repo format)")
	_ = c.MarkFlagRequired("repo")
	_ = c.MarkFlagRequired("ref")
	_ = c.MarkFlagRequired("out")
	return c
}

func newExportBatchCmd(resolveServer func() (string, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "export-batch",
		Short: "Export multiple repo/ref entries from a YAML or JSON array spec",
		RunE: func(cmd *cobra.Command, _ []string) error {
			server, err := resolveServer()
			if err != nil {
				return err
			}
			specPath, _ := cmd.Flags().GetString("spec")
			out, _ := cmd.Flags().GetString("out")
			format, _ := cmd.Flags().GetString("format")

			data, err := os.ReadFile(specPath)
			if err != nil {
				return err
			}
			items, err := parseBatchSpec(data)
			if err != nil {
				return err
			}
			logf := func(format string, a ...any) { fmt.Fprintf(os.Stderr, format+"\n", a...) }
			httpClient, err := httpClientFromCmd(cmd)
			if err != nil {
				return err
			}
			for i, it := range items {
				if it.Repository == "" || it.Ref == "" {
					logf("batch[%d]: skip (missing repository or ref)", i)
					continue
				}
				files, err := apiclient.ExportContracts(context.Background(), httpClient, server, apiclient.ExportRequest{
					Repository:   it.Repository,
					Ref:          it.Ref,
					Path:         it.Path,
					ContractName: it.Contract,
				})
				if err != nil {
					logf("batch[%d] %s@%s: %v", i, it.Repository, it.Ref, err)
					continue
				}
				if err := outfiles.WriteExported(files, out, format); err != nil {
					logf("batch[%d] write: %v", i, err)
				}
			}
			return nil
		},
	}
	c.Flags().String("spec", "", "path to YAML/JSON file with array of {repository, ref, path?, contract?}")
	c.Flags().String("out", "", "output directory")
	c.Flags().String("format", "", "optional: yaml or json for all items")
	_ = c.MarkFlagRequired("spec")
	_ = c.MarkFlagRequired("out")
	return c
}

func newDiffCmd(resolveServer func() (string, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "diff",
		Short: "Compare local contract file(s) with files exported from git (same flags as export, plus --target)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			server, err := resolveServer()
			if err != nil {
				return err
			}
			repo, _ := cmd.Flags().GetString("repo")
			ref, _ := cmd.Flags().GetString("ref")
			path, _ := cmd.Flags().GetString("path")
			contract, _ := cmd.Flags().GetString("contract")
			target, _ := cmd.Flags().GetString("target")

			httpClient, err := httpClientFromCmd(cmd)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			color := style.UseColorFor(out)
			_, err = contractdiff.Compare(context.Background(), contractdiff.Options{
				Target:     target,
				ServerURL:  server,
				HTTPClient: httpClient,
				Request: apiclient.ExportRequest{
					Repository:   repo,
					Ref:          ref,
					Path:         path,
					ContractName: contract,
				},
				Out:   out,
				Color: color,
			})
			return err
		},
	}
	c.Flags().String("repo", "", "repository name (as in contract-syncer config)")
	c.Flags().String("ref", "", "git ref (e.g. heads/main, remotes/origin/master)")
	c.Flags().String("path", "", "path inside repository (omit for entire repo)")
	c.Flags().String("contract", "", "single contract name (omit for all under path)")
	c.Flags().String("target", "", "local file or directory to compare (same role as export --out)")
	_ = c.MarkFlagRequired("repo")
	_ = c.MarkFlagRequired("ref")
	_ = c.MarkFlagRequired("target")
	return c
}

func newDiffBatchCmd(resolveServer func() (string, error)) *cobra.Command {
	c := &cobra.Command{
		Use:   "diff-batch",
		Short: "Compare multiple repo/ref entries using the same spec format as export-batch",
		RunE: func(cmd *cobra.Command, _ []string) error {
			server, err := resolveServer()
			if err != nil {
				return err
			}
			specPath, _ := cmd.Flags().GetString("spec")
			target, _ := cmd.Flags().GetString("target")
			data, err := os.ReadFile(specPath)
			if err != nil {
				return err
			}
			items, err := parseBatchSpec(data)
			if err != nil {
				return err
			}
			logf := func(format string, a ...any) { fmt.Fprintf(os.Stderr, format+"\n", a...) }
			httpClient, err := httpClientFromCmd(cmd)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			color := style.UseColorFor(out)
			var anyDiff bool
			var printed bool
			for i, it := range items {
				if it.Repository == "" || it.Ref == "" {
					logf("diff-batch[%d]: skip (missing repository or ref)", i)
					continue
				}
				if printed {
					fmt.Fprintln(out)
					fmt.Fprintln(out, strings.Repeat("─", 42))
				}
				printed = true
				fmt.Fprintf(out, "%s %s @ %s → %s\n",
					style.S(color, style.Dim, fmt.Sprintf("batch[%d]", i)),
					it.Repository, it.Ref, target)
				_, err := contractdiff.Compare(context.Background(), contractdiff.Options{
					Target:     target,
					ServerURL:  server,
					HTTPClient: httpClient,
					Request: apiclient.ExportRequest{
						Repository:   it.Repository,
						Ref:          it.Ref,
						Path:         it.Path,
						ContractName: it.Contract,
					},
					Out:   out,
					Color: color,
				})
				if err != nil {
					if errors.Is(err, contractdiff.ErrChanges) {
						anyDiff = true
						continue
					}
					logf("diff-batch[%d] %s@%s: %v", i, it.Repository, it.Ref, err)
				}
			}
			if anyDiff {
				return contractdiff.ErrChanges
			}
			return nil
		},
	}
	c.Flags().String("spec", "", "path to YAML/JSON file with array of {repository, ref, path?, contract?} (same as export-batch)")
	c.Flags().String("target", "", "local file or directory to compare (same role as export-batch --out)")
	_ = c.MarkFlagRequired("spec")
	_ = c.MarkFlagRequired("target")
	return c
}

type batchItem struct {
	Repository string `json:"repository" yaml:"repository"`
	Ref        string `json:"ref" yaml:"ref"`
	Path       string `json:"path,omitempty" yaml:"path,omitempty"`
	Contract   string `json:"contract,omitempty" yaml:"contract,omitempty"`
}

func parseBatchSpec(data []byte) ([]batchItem, error) {
	var items []batchItem
	if err := yaml.Unmarshal(data, &items); err != nil {
		if err2 := json.Unmarshal(data, &items); err2 != nil {
			return nil, fmt.Errorf("parse spec: yaml: %w; json: %v", err, err2)
		}
	}
	return items, nil
}

func httpClientFromCmd(cmd *cobra.Command) (*http.Client, error) {
	insecure, err := cmd.Flags().GetBool("insecure")
	if err != nil {
		return nil, err
	}
	caPath, err := cmd.Flags().GetString("ca-cert")
	if err != nil {
		return nil, err
	}
	return apiclient.NewHTTPClient(apiclient.TLSOptions{
		Insecure:   insecure,
		CACertPath: strings.TrimSpace(caPath),
	})
}
