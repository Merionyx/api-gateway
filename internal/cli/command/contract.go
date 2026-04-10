// Package command registers cobra subcommands for agwctl.
package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/merionyx/api-gateway/internal/cli/apiclient"
	"github.com/merionyx/api-gateway/internal/cli/outfiles"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewContractCommand builds `agwctl contract ...`. resolveServer returns the API Server base URL.
func NewContractCommand(resolveServer func() (string, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contract",
		Short: "Contract schema files",
	}
	cmd.AddCommand(newExportCmd(resolveServer), newExportBatchCmd(resolveServer))
	return cmd
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

			files, err := apiclient.ExportContracts(context.Background(), server, apiclient.ExportRequest{
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
			var items []batchItem
			if err := yaml.Unmarshal(data, &items); err != nil {
				if err2 := json.Unmarshal(data, &items); err2 != nil {
					return fmt.Errorf("parse spec: yaml: %w; json: %v", err, err2)
				}
			}
			logf := func(format string, a ...any) { fmt.Fprintf(os.Stderr, format+"\n", a...) }
			for i, it := range items {
				if it.Repository == "" || it.Ref == "" {
					logf("batch[%d]: skip (missing repository or ref)", i)
					continue
				}
				files, err := apiclient.ExportContracts(context.Background(), server, apiclient.ExportRequest{
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

type batchItem struct {
	Repository string `json:"repository" yaml:"repository"`
	Ref        string `json:"ref" yaml:"ref"`
	Path       string `json:"path,omitempty" yaml:"path,omitempty"`
	Contract   string `json:"contract,omitempty" yaml:"contract,omitempty"`
}
