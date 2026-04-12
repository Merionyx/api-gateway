// Package command registers cobra subcommands for agwctl.
package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	apiclient "github.com/merionyx/api-gateway/internal/cli/apiserver"
	"github.com/merionyx/api-gateway/internal/cli/contractdiff"
	"github.com/merionyx/api-gateway/internal/cli/contractfmt"
	"github.com/merionyx/api-gateway/internal/cli/outfiles"
	"github.com/merionyx/api-gateway/internal/cli/style"
	"github.com/merionyx/api-gateway/internal/cli/validate"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ErrFmtCheck is returned when contract fmt --check finds files that still need formatting
// or could not be read/formatted (see stdout for the styled report).
var ErrFmtCheck = errors.New("fmt check: not clean")

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
	var check bool
	var format string
	c := &cobra.Command{
		Use:   "fmt PATH",
		Short: "Canonicalize contract YAML/JSON (OpenAPI 3.x via kin-openapi, x-api-gateway v1 with fixed field order)",
		Long: "Without --write, prints the formatted document to stdout (single file only).\n" +
			"With --write, overwrites source file(s). With --check, compares each file to formatted output " +
			"and prints a report (same style as contract diff); exits with error if anything would change or failed to read/format. " +
			"Directories require --write or --check.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			if write && check {
				return fmt.Errorf("--write and --check cannot be used together")
			}
			fi, err := os.Stat(filepath.Clean(target))
			if err != nil {
				return err
			}
			if fi.IsDir() && !write && !check {
				return fmt.Errorf("formatting a directory requires --write or --check")
			}

			paths, err := validate.CollectFiles(target)
			if err != nil {
				return err
			}
			if len(paths) == 0 {
				return fmt.Errorf("no .yaml/.yml/.json files under %s", target)
			}

			out := cmd.OutOrStdout()
			logErr := func(format string, a ...any) { _, _ = fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", a...) }
			var nwrite, nfail int
			var needsFormat, okPaths, failedPaths []string
			for _, p := range paths {
				data, err := os.ReadFile(p)
				if err != nil {
					logErr("%s: read: %v", p, err)
					failedPaths = append(failedPaths, p)
					nfail++
					continue
				}
				ext := filepath.Ext(p)
				formatted, err := contractfmt.FormatBytes(data, ext, format)
				if err != nil {
					logErr("%s: %v", p, err)
					failedPaths = append(failedPaths, p)
					nfail++
					continue
				}
				if check {
					if !bytes.Equal(data, formatted) {
						needsFormat = append(needsFormat, p)
					} else {
						okPaths = append(okPaths, p)
					}
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
					_, _ = fmt.Fprintf(out, "# %s\n", p)
				}
				if _, err := out.Write(formatted); err != nil {
					return err
				}
				if len(paths) > 1 {
					_, _ = fmt.Fprintln(out)
				}
			}
			if check {
				color := style.UseColorFor(out)
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprintln(out, style.S(color, style.Bold, "Format check"))
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprintf(out, "%s %s\n", style.S(color, style.Dim, "Target:"), style.S(color, style.Bold, target))
				_, _ = fmt.Fprintln(out)

				sort.Strings(paths)
				needSet := make(map[string]struct{}, len(needsFormat))
				for _, p := range needsFormat {
					needSet[p] = struct{}{}
				}
				failSet := make(map[string]struct{}, len(failedPaths))
				for _, p := range failedPaths {
					failSet[p] = struct{}{}
				}
				for _, p := range paths {
					switch {
					case containsStrSet(failSet, p):
						_, _ = fmt.Fprintf(out, "%s %s %s %s\n",
							style.S(color, style.Red, markFail),
							style.S(color, style.Dim, "File:"),
							style.S(color, style.Bold, p),
							style.S(color, style.Red, "[failed]"),
						)
					case containsStrSet(needSet, p):
						_, _ = fmt.Fprintf(out, "%s %s %s %s\n",
							style.S(color, style.Yellow, "!"),
							style.S(color, style.Dim, "File:"),
							style.S(color, style.Bold, p),
							style.S(color, style.Yellow, "[needs formatting]"),
						)
					default:
						_, _ = fmt.Fprintf(out, "%s %s %s %s\n",
							style.S(color, style.Green, markOK),
							style.S(color, style.Dim, "File:"),
							style.S(color, style.Bold, p),
							style.S(color, style.Green, "[ok]"),
						)
					}
				}
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprintln(out, fmtCheckSummaryLine(len(okPaths), len(needsFormat), len(failedPaths), color))
				if nfail > 0 || len(needsFormat) > 0 {
					_, _ = fmt.Fprintln(out)
					return ErrFmtCheck
				}
				return nil
			}
			if write && fi.IsDir() {
				out := cmd.OutOrStdout()
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprintf(out, "%s %s %d file(s) %s %s\n", style.S(style.UseColorFor(out), style.Green, markOK), style.S(style.UseColorFor(out), style.Dim, "formatted"), nwrite, style.S(style.UseColorFor(out), style.Dim, "under"), target)
			}
			if nfail > 0 {
				return fmt.Errorf("format failed for %d file(s)", nfail)
			}
			return nil
		},
	}
	c.Flags().BoolVarP(&write, "write", "w", false, "write result to source file(s) instead of stdout")
	c.Flags().BoolVar(&check, "check", false, "verify files are already formatted (styled report like contract diff); exit non-zero if any would change")
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
			req := apiclient.ExportRequest{
				Repository:   repo,
				Ref:          ref,
				Path:         path,
				ContractName: contract,
			}
			files, err := apiclient.ExportContracts(context.Background(), httpClient, server, req)
			if err != nil {
				return err
			}
			if err := outfiles.WriteExported(files, out, format); err != nil {
				return err
			}
			outw := cmd.OutOrStdout()
			printExportContractsReport(outw, style.UseColorFor(outw), req, out, files)
			return nil
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
		Short: "Export multiple repo/ref entries from a YAML or JSON array spec (--out or per-entry out)",
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
			if err := validateBatchDestOrSpec(out, items); err != nil {
				return err
			}
			logf := func(format string, a ...any) { fmt.Fprintf(os.Stderr, format+"\n", a...) }
			httpClient, err := httpClientFromCmd(cmd)
			if err != nil {
				return err
			}
			stdout := cmd.OutOrStdout()
			color := style.UseColorFor(stdout)
			var printed bool
			_, _ = fmt.Fprintln(stdout)
			for i, it := range items {
				if it.Repository == "" || it.Ref == "" {
					logf("batch[%d]: skip (missing repository or ref)", i)
					continue
				}
				dest := batchItemDest(out, it)
				req := apiclient.ExportRequest{
					Repository:   it.Repository,
					Ref:          it.Ref,
					Path:         it.Path,
					ContractName: batchItemExportContract(it),
				}
				files, err := apiclient.ExportContracts(context.Background(), httpClient, server, req)
				if err != nil {
					logf("batch[%d] %s@%s: %v", i, it.Repository, it.Ref, err)
					continue
				}
				if err := outfiles.WriteExported(files, dest, format); err != nil {
					logf("batch[%d] write: %v", i, err)
					continue
				}
				if printed {
					_, _ = fmt.Fprintln(stdout)
				}
				printed = true
				_, _ = fmt.Fprintf(stdout, "%s %s @ %s → %s\n",
					style.S(color, style.Dim, fmt.Sprintf("batch[%d]", i)),
					it.Repository, it.Ref, dest)
				printExportContractsReport(stdout, color, req, dest, files)
			}
			return nil
		},
	}
	c.Flags().String("spec", "", "path to YAML/JSON file with array of {repository, ref, path?, contract or contract_name?, out?}")
	c.Flags().String("out", "", "output directory for all entries (if omitted, each spec entry must have \"out\")")
	c.Flags().String("format", "", "optional: yaml or json for all items")
	_ = c.MarkFlagRequired("spec")
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
		Short: "Compare multiple repo/ref entries using the same spec as export-batch (--target or per-entry out)",
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
			if err := validateBatchDestOrSpec(target, items); err != nil {
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
				itemTarget := batchItemDest(target, it)
				if printed {
					_, _ = fmt.Fprintln(out)
				}
				printed = true
				_, _ = fmt.Fprintf(out, "%s %s @ %s → %s\n",
					style.S(color, style.Dim, fmt.Sprintf("batch[%d]", i)),
					it.Repository, it.Ref, itemTarget)
				_, err := contractdiff.Compare(context.Background(), contractdiff.Options{
					Target:     itemTarget,
					ServerURL:  server,
					HTTPClient: httpClient,
					Request: apiclient.ExportRequest{
						Repository:   it.Repository,
						Ref:          it.Ref,
						Path:         it.Path,
						ContractName: batchItemExportContract(it),
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
	c.Flags().String("spec", "", "path to YAML/JSON file with array of {repository, ref, path?, contract or contract_name?, out?} (same as export-batch)")
	c.Flags().String("target", "", "local path for all entries (if omitted, each spec entry must have \"out\")")
	_ = c.MarkFlagRequired("spec")
	return c
}

type batchItem struct {
	Repository string `json:"repository" yaml:"repository"`
	Ref        string `json:"ref" yaml:"ref"`
	Path       string `json:"path,omitempty" yaml:"path,omitempty"`
	// Contract is the short key; ContractName matches the export API JSON field contract_name.
	Contract     string `json:"contract,omitempty" yaml:"contract,omitempty"`
	ContractName string `json:"contract_name,omitempty" yaml:"contract_name,omitempty"`
	// Out is the output directory (export-batch) or compare target (diff-batch) when --out / --target is not set.
	Out string `json:"out,omitempty" yaml:"out,omitempty"`
}

// batchItemExportContract returns the optional single-contract filter for export/diff-batch.
// If both contract and contract_name are set, contract wins.
func batchItemExportContract(it batchItem) string {
	if s := strings.TrimSpace(it.Contract); s != "" {
		return s
	}
	return strings.TrimSpace(it.ContractName)
}

// validateBatchDestOrSpec requires either a non-empty global dest (--out / --target) or a non-empty "out" on every spec item.
func validateBatchDestOrSpec(globalDest string, items []batchItem) error {
	if strings.TrimSpace(globalDest) != "" {
		return nil
	}
	var missing []int
	for i, it := range items {
		if strings.TrimSpace(it.Out) == "" {
			missing = append(missing, i)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("either set --out/--target, or give a non-empty \"out\" on every spec entry (missing at indices %v)", missing)
	}
	return nil
}

// batchItemDest returns the directory/target for one entry: global flag wins when set, otherwise the item's "out".
func batchItemDest(globalDest string, it batchItem) string {
	if g := strings.TrimSpace(globalDest); g != "" {
		return g
	}
	return strings.TrimSpace(it.Out)
}

func printExportKV(w io.Writer, color bool, k, v string) {
	_, _ = fmt.Fprintf(w, "%s %s\n", style.S(color, style.Dim, k+":"), style.S(color, style.Bold, v))
}

// printExportContractsReport prints a diff-style report after a successful export (stdout).
func printExportContractsReport(w io.Writer, color bool, req apiclient.ExportRequest, outDir string, files []apiclient.ExportFile) {
	sorted := append([]apiclient.ExportFile(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ContractName < sorted[j].ContractName })

	_, _ = fmt.Fprintln(w)

	for _, f := range sorted {
		_, _ = fmt.Fprintf(w, "%s %s %s\n",
			style.S(color, style.Green, markOK),
			style.S(color, style.Dim, "Contract:"),
			style.S(color, style.Bold, f.ContractName),
		)
	}
}

func containsStrSet(m map[string]struct{}, k string) bool {
	_, ok := m[k]
	return ok
}

// fmtCheckSummaryLine matches the contract diff footer tone (e.g. "4 files · ok" or "2 ok · 1 need formatting").
func fmtCheckSummaryLine(nOk, nNeeds, nFail int, color bool) string {
	dot := style.S(color, style.Dim, "·")
	if nNeeds == 0 && nFail == 0 {
		ok := style.S(color, style.Green, "ok")
		if nOk == 1 {
			return fmt.Sprintf("1 file %s %s", dot, ok)
		}
		return fmt.Sprintf("%d files %s %s", nOk, dot, ok)
	}
	var parts []string
	if nOk > 0 {
		parts = append(parts, style.S(color, style.Gray, fmt.Sprintf("%d ok", nOk)))
	}
	if nNeeds > 0 {
		parts = append(parts, style.S(color, style.Yellow, fmt.Sprintf("%d need formatting", nNeeds)))
	}
	if nFail > 0 {
		parts = append(parts, style.S(color, style.Red, fmt.Sprintf("%d failed", nFail)))
	}
	return strings.Join(parts, fmt.Sprintf(" %s ", dot))
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
