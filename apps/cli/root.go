package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/sandeepv/hoptrace/internal/platform"
	"github.com/sandeepv/hoptrace/internal/version"
)

func main() {
	platform.SetLogLevel(slog.LevelWarn)
	if os.Getenv("HOPTRACE_VERBOSE") == "1" {
		platform.SetLogLevel(slog.LevelInfo)
	}

	root := newRootCmd()
	if err := root.Execute(); err != nil {
		os.Exit(platform.ExitCodeOf(err))
	}
}

func newRootCmd() *cobra.Command {
	var f probeFlags
	verbose := false

	root := &cobra.Command{
		Use:   "hoptrace [url]",
		Short: "HTTP latency profiler — DNS, connect, TLS, wait, transfer",
		Long: `hoptrace probes a URL and breaks timing into DNS, connect, TLS, wait, and transfer.

Examples:
  hoptrace https://example.com
  hoptrace save stance-patients 'https://dashboard.stance.health/patients'
  hoptrace run stance-patients
  hoptrace saved list
  hoptrace history`,
		Args: cobra.MaximumNArgs(1),
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			if verbose {
				platform.SetLogLevel(slog.LevelInfo)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			f.proxySet = cmd.Flags().Changed("proxy")
			return runProbe(cmd, args[0], f)
		},
	}

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show probe debug logs on stderr")
	bindProbeFlags(root, &f)

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "hoptrace", version.Version)
		},
	})
	root.AddCommand(newCompletionCmd())

	probeCmd := &cobra.Command{
		Use:   "probe [url]",
		Short: "Alias for probing a URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f.proxySet = cmd.Flags().Changed("proxy")
			return runProbe(cmd, args[0], f)
		},
	}
	bindProbeFlags(probeCmd, &f)
	root.AddCommand(probeCmd)
	root.AddCommand(newHistoryCmd())
	root.AddCommand(newSaveCmd(&f))
	root.AddCommand(newRunCmd(&f))
	root.AddCommand(newSavedCmd())
	root.AddCommand(newScheduleCmd())
	root.AddCommand(newKeysCmd())
	root.AddCommand(newBaselineCmd())
	root.AddCommand(newDashboardCmd())

	root.SilenceUsage = true
	root.SilenceErrors = true
	return root
}
