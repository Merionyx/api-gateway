package command

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/merionyx/api-gateway/internal/cli/config"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewConfigCommand builds `agwctl config ...`.
func NewConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and edit agwctl contexts (~/.config/agwctl/config.yaml)",
	}
	cmd.AddCommand(
		newConfigGetContextsCmd(),
		newConfigCurrentContextCmd(),
		newConfigUseContextCmd(),
		newConfigSetContextCmd(),
		newConfigDeleteContextCmd(),
		newConfigViewCmd(),
	)
	return cmd
}

func newConfigGetContextsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get-contexts",
		Short: "List all contexts and mark the current one",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			names := cfg.ContextNames()
			if len(names) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No contexts. Use: agwctl config set-context NAME --server URL")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "CURRENT\tNAME\tSERVER")
			cur := strings.TrimSpace(cfg.CurrentContext)
			for _, name := range names {
				mark := ""
				if cur != "" && name == cur {
					mark = "*"
				}
				srv := ""
				if c, ok := cfg.Contexts[name]; ok {
					srv = c.Server
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", mark, name, srv)
			}
			return w.Flush()
		},
	}
}

func newConfigCurrentContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current-context",
		Short: "Print the current context name",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			name := strings.TrimSpace(cfg.CurrentContext)
			if name == "" {
				return fmt.Errorf("no current context (use: agwctl config use-context NAME or set-context with a new context)")
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), name)
			return err
		},
	}
}

func newConfigUseContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use-context CONTEXT_NAME",
		Short: "Switch the current context",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := cfg.UseContext(args[0]); err != nil {
				return err
			}
			return config.Save(cfg)
		},
	}
}

func newConfigSetContextCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "set-context CONTEXT_NAME",
		Short: "Create or update a context with --server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			server, _ := cmd.Flags().GetString("server")
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := cfg.SetContext(args[0], server); err != nil {
				return err
			}
			return config.Save(cfg)
		},
	}
	c.Flags().String("server", "", "API Server base URL, e.g. http://127.0.0.1:8080")
	_ = c.MarkFlagRequired("server")
	return c
}

func newConfigDeleteContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete-context CONTEXT_NAME",
		Short: "Remove a context (clears current-context if it was deleted)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			ok, err := cfg.DeleteContext(args[0])
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("context %q not found", strings.TrimSpace(args[0]))
			}
			return config.Save(cfg)
		},
	}
}

func newConfigViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Print the raw config file (same path as $AGWCTL_CONFIG or default)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := config.Path()
			if err != nil {
				return err
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "# %s\n%s", p, data)
			return err
		},
	}
}
