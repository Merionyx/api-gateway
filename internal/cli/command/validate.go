package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/merionyx/api-gateway/internal/cli/style"
	"github.com/merionyx/api-gateway/internal/cli/validate"

	"github.com/spf13/cobra"
)

const (
	markOK     = "\u2713" // check mark
	markFail   = "\u2717" // ballot X
	arrowRight = "\u27A1" // right arrow
)

// NewValidateCommand builds `agwctl validate PATH` (file or directory of contract YAML/JSON).
func NewValidateCommand() *cobra.Command {
	var checkContent bool
	cmd := &cobra.Command{
		Use:   "validate PATH",
		Short: "Validate x-api-gateway metadata (v1) and optionally OpenAPI document bodies",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := validate.Options{CheckContent: checkContent}
			results := validate.Run(context.Background(), args[0], opts)
			out := cmd.OutOrStdout()
			color := style.UseColorFor(out)

			fmt.Fprintln(out)
			fmt.Fprintln(out, style.S(color, style.Bold, "Validate contracts"))
			fmt.Fprintln(out)

			var totalIssues int
			for _, r := range results {
				fileLabel := style.S(color, style.Dim, "File:")
				pathBold := style.S(color, style.Bold, r.Path)

				if len(r.Issues) == 0 {
					line := fmt.Sprintf("%s %s %s",
						style.S(color, style.Green, markOK),
						fileLabel,
						pathBold,
					)
					fmt.Fprintln(out, line)
					continue
				}
				totalIssues += len(r.Issues)
				line := fmt.Sprintf("%s %s %s %s",
					style.S(color, style.Red, markFail),
					fileLabel,
					pathBold,
					style.S(color, style.Red, "[failed]"),
				)
				fmt.Fprintln(out, line)
				for _, msg := range r.Issues {
					prefix := fmt.Sprintf("      %s ", style.S(color, style.Dim, arrowRight))
					fmt.Fprintln(out, prefix+style.S(color, style.Dim, msg))
				}
			}

			fmt.Fprintln(out)
			var files, status string
			switch {
			case len(results) == 0:
				files = "0 files"
			case totalIssues == 0:
				files = fmt.Sprintf("%d files", len(results))
				status = style.S(color, style.Green, "ok")
			default:
				files = fmt.Sprintf("%d files", len(results))
				status = style.S(color, style.Red, fmt.Sprintf("%d issues", totalIssues))
			}
			fmt.Fprintln(out, files, style.S(color, style.Dim, "·"), status)

			if totalIssues > 0 {
				fmt.Fprintln(out)
				return errors.New("validation failed")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&checkContent, "check-content", false, "also validate document content (OpenAPI 3.x when openapi field is present); more checkers may be added later")
	return cmd
}
