package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sandeepv/hoptrace/internal/export"
	"github.com/sandeepv/hoptrace/internal/platform"
	"github.com/sandeepv/hoptrace/internal/probe"
	"github.com/sandeepv/hoptrace/internal/repository"
	"github.com/sandeepv/hoptrace/internal/setup"
	"github.com/sandeepv/hoptrace/internal/slo"
	"github.com/sandeepv/hoptrace/internal/version"
)

func main() {
	var f probeFlags
	verbose := false

	// Quiet by default so the waterfall stays readable.
	platform.SetLogLevel(slog.LevelWarn)
	if os.Getenv("HOPTRACE_VERBOSE") == "1" {
		platform.SetLogLevel(slog.LevelInfo)
	}

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
	if err := root.Execute(); err != nil {
		if _, ok := err.(*platform.UsageError); ok {
			os.Exit(platform.ExitUsage)
		}
		os.Exit(platform.ExitSoftware)
	}
}

func bindProbeFlags(cmd *cobra.Command, f *probeFlags) {
	cmd.Flags().StringVarP(&f.method, "method", "X", "", "HTTP method")
	cmd.Flags().StringArrayVarP(&f.headers, "header", "H", nil, "Request header (repeatable)")
	cmd.Flags().StringVarP(&f.data, "data", "d", "", "Request body (or @file)")
	cmd.Flags().BoolVarP(&f.follow, "follow", "L", false, "Follow redirects")
	cmd.Flags().Float64VarP(&f.timeoutSec, "timeout", "m", 30, "Timeout in seconds")
	cmd.Flags().StringVarP(&f.proxy, "proxy", "x", "", "Proxy URL (empty disables env proxy)")
	cmd.Flags().BoolVarP(&f.ignoreSSL, "ignore-ssl", "k", false, "Skip TLS verification")
	cmd.Flags().StringVar(&f.cacert, "cacert", "", "Custom CA bundle path")
	cmd.Flags().BoolVar(&f.compact, "compact", false, "Compact one-line output")
	cmd.Flags().BoolVar(&f.metricsOnly, "metrics-only", false, "Metrics-only scripting output")
	cmd.Flags().StringVar(&f.jsonPath, "json", "", "Write JSON report to path")
	cmd.Flags().StringVar(&f.sloSpec, "slo", "", "SLO thresholds e.g. total=500,ttfb=200")
	cmd.Flags().BoolVar(&f.noSave, "no-save", false, "Do not persist this run to history")
}

type probeFlags struct {
	method, data, proxy, cacert, jsonPath, sloSpec string
	headers                                        []string
	follow, ignoreSSL, compact, metricsOnly        bool
	timeoutSec                                     float64
	proxySet, noSave                               bool
}

func runProbe(cmd *cobra.Command, rawURL string, f probeFlags) error {
	b := probe.NewRequestBuilder().URL(rawURL)
	if f.method != "" {
		b.Method(f.method)
	}
	for _, h := range f.headers {
		k, v, ok := strings.Cut(h, ":")
		if !ok {
			return usage("invalid header %q (want Key: Value)", h)
		}
		b.Header(strings.TrimSpace(k), strings.TrimSpace(v))
	}
	if f.data != "" {
		body, err := readData(f.data)
		if err != nil {
			return usage("%v", err)
		}
		b.Body(body)
	}
	b.FollowRedirect(f.follow)
	b.Timeout(time.Duration(f.timeoutSec * float64(time.Second)))
	if f.ignoreSSL {
		b.IgnoreSSL(true)
	}
	if f.cacert != "" {
		b.CABundle(f.cacert)
	}
	if f.proxySet {
		b.Proxy(f.proxy)
	}
	if f.sloSpec != "" {
		thresholds, err := slo.ParseSpec(f.sloSpec)
		if err != nil {
			return usage("%v", err)
		}
		b.SLO(thresholds)
	}

	req, err := b.Build()
	if err != nil {
		return usage("%v", err)
	}

	mode := "waterfall"
	switch {
	case f.metricsOnly:
		mode = "metrics-only"
	case f.compact:
		mode = "compact"
	case f.jsonPath != "":
		mode = "json"
	}

	exporter, err := export.Factory(mode, cmd.OutOrStdout(), f.jsonPath)
	if err != nil {
		return usage("%v", err)
	}

	report, probeErr := setup.DefaultAnalyzer().Analyze(context.Background(), req)
	_ = exporter.Export(report)
	if mode != "json" && f.jsonPath != "" {
		je, _ := export.Factory("json", cmd.OutOrStdout(), f.jsonPath)
		_ = je.Export(report)
	}

	if !f.noSave {
		if path, err := repository.DefaultDBPath(); err == nil {
			if repo, err := repository.OpenSQLite(path); err == nil {
				if rec, err := repo.Save(context.Background(), report); err == nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "saved history id=%s\n", rec.ID)
				}
				_ = repo.Close()
			}
		}
	}

	if probeErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "error: %v\n", probeErr)
		os.Exit(platform.ExitTempFail)
	}
	if report.Summary.SLO != nil && !report.Summary.SLO.Pass {
		fmt.Fprintf(cmd.ErrOrStderr(), "slo failed (exit %d)\n", platform.ExitSLOFail)
		os.Exit(platform.ExitSLOFail)
	}
	return nil
}

func newHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "List recent probe runs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			repo, err := openHistory()
			if err != nil {
				return err
			}
			defer repo.Close()
			list, err := repo.List(context.Background(), 25)
			if err != nil {
				return err
			}
			if len(list) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no history yet")
				return nil
			}
			for _, rec := range list {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  status=%-3d  %7.1fms  %s\n",
					rec.ID,
					rec.Report.Summary.FinalStatus,
					rec.Report.Summary.TotalTimeMS,
					rec.Report.InitialURL,
				)
			}
			return nil
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "show [id]",
		Short: "Show a historical probe report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := openHistory()
			if err != nil {
				return err
			}
			defer repo.Close()
			rec, err := repo.Get(context.Background(), args[0])
			if err != nil {
				return err
			}
			exp, _ := export.Factory("waterfall", cmd.OutOrStdout(), "")
			return exp.Export(rec.Report)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "compare [id-a] [id-b]",
		Short: "Compare two historical runs",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := openHistory()
			if err != nil {
				return err
			}
			defer repo.Close()
			a, err := repo.Get(context.Background(), args[0])
			if err != nil {
				return fmt.Errorf("id-a: %w", err)
			}
			b, err := repo.Get(context.Background(), args[1])
			if err != nil {
				return fmt.Errorf("id-b: %w", err)
			}
			printCompare(cmd, a, b)
			return nil
		},
	})

	return cmd
}

func printCompare(cmd *cobra.Command, a, b repository.ProbeRecord) {
	fmt.Fprintf(cmd.OutOrStdout(), "Compare\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  A %s  %.1fms  status=%d  %s\n", a.ID, a.Report.Summary.TotalTimeMS, a.Report.Summary.FinalStatus, a.Report.InitialURL)
	fmt.Fprintf(cmd.OutOrStdout(), "  B %s  %.1fms  status=%d  %s\n", b.ID, b.Report.Summary.TotalTimeMS, b.Report.Summary.FinalStatus, b.Report.InitialURL)
	delta := b.Report.Summary.TotalTimeMS - a.Report.Summary.TotalTimeMS
	fmt.Fprintf(cmd.OutOrStdout(), "  delta total: %+.1f ms\n", delta)
	if len(a.Report.Steps) > 0 && len(b.Report.Steps) > 0 {
		sa, sb := a.Report.Steps[len(a.Report.Steps)-1].Timing, b.Report.Steps[len(b.Report.Steps)-1].Timing
		fmt.Fprintf(cmd.OutOrStdout(), "  dns     %7.1f -> %7.1f  (%+.1f)\n", sa.DNSMS, sb.DNSMS, sb.DNSMS-sa.DNSMS)
		fmt.Fprintf(cmd.OutOrStdout(), "  connect %7.1f -> %7.1f  (%+.1f)\n", sa.ConnectMS, sb.ConnectMS, sb.ConnectMS-sa.ConnectMS)
		fmt.Fprintf(cmd.OutOrStdout(), "  tls     %7.1f -> %7.1f  (%+.1f)\n", sa.TLSMS, sb.TLSMS, sb.TLSMS-sa.TLSMS)
		fmt.Fprintf(cmd.OutOrStdout(), "  wait    %7.1f -> %7.1f  (%+.1f)\n", sa.WaitMS, sb.WaitMS, sb.WaitMS-sa.WaitMS)
		fmt.Fprintf(cmd.OutOrStdout(), "  xfer    %7.1f -> %7.1f  (%+.1f)\n", sa.XferMS, sb.XferMS, sb.XferMS-sa.XferMS)
		fmt.Fprintf(cmd.OutOrStdout(), "  ttfb    %7.1f -> %7.1f  (%+.1f)\n", sa.TTFBMS, sb.TTFBMS, sb.TTFBMS-sa.TTFBMS)
	}
}

func newSaveCmd(f *probeFlags) *cobra.Command {
	var desc string
	cmd := &cobra.Command{
		Use:   "save [name] [url]",
		Short: "Save a named probe template",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			f.proxySet = cmd.Flags().Changed("proxy")
			method := f.method
			if method == "" {
				method = "GET"
			}
			headers := map[string]string{}
			for _, h := range f.headers {
				k, v, ok := strings.Cut(h, ":")
				if !ok {
					return usage("invalid header %q", h)
				}
				headers[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
			sp := repository.SavedProbe{
				Name:            strings.TrimSpace(args[0]),
				Description:     desc,
				URL:             args[1],
				Method:          method,
				Headers:         headers,
				Body:            f.data,
				FollowRedirects: f.follow,
				TimeoutMS:       int(f.timeoutSec * 1000),
				IgnoreSSL:       f.ignoreSSL,
			}
			if f.proxySet {
				p := f.proxy
				sp.Proxy = &p
			}
			if f.sloSpec != "" {
				m, err := slo.ParseSpec(f.sloSpec)
				if err != nil {
					return usage("%v", err)
				}
				sp.SLO = m
			}
			repo, err := openHistory()
			if err != nil {
				return err
			}
			defer repo.Close()
			out, err := repo.UpsertSaved(context.Background(), sp)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved %s -> %s\n", out.Name, out.URL)
			return nil
		},
	}
	bindProbeFlags(cmd, f)
	cmd.Flags().StringVar(&desc, "desc", "", "Optional description")
	return cmd
}

func newRunCmd(f *probeFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [name]",
		Short: "Run a saved probe template by name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := openHistory()
			if err != nil {
				return err
			}
			defer repo.Close()
			sp, err := repo.GetSaved(context.Background(), args[0])
			if err != nil {
				return err
			}
			flags := *f
			flags.proxySet = cmd.Flags().Changed("proxy")
			if !cmd.Flags().Changed("method") || flags.method == "" {
				flags.method = sp.Method
			}
			if !cmd.Flags().Changed("follow") {
				flags.follow = sp.FollowRedirects
			}
			if !cmd.Flags().Changed("data") && flags.data == "" {
				flags.data = sp.Body
			}
			if !cmd.Flags().Changed("timeout") && sp.TimeoutMS > 0 {
				flags.timeoutSec = float64(sp.TimeoutMS) / 1000.0
			}
			if len(flags.headers) == 0 {
				for k, v := range sp.Headers {
					flags.headers = append(flags.headers, k+": "+v)
				}
			}
			if flags.sloSpec == "" && len(sp.SLO) > 0 {
				parts := make([]string, 0, len(sp.SLO))
				for k, v := range sp.SLO {
					parts = append(parts, fmt.Sprintf("%s=%g", k, v))
				}
				flags.sloSpec = strings.Join(parts, ",")
			}
			if !flags.proxySet && sp.Proxy != nil {
				flags.proxy = *sp.Proxy
				flags.proxySet = true
			}
			flags.ignoreSSL = flags.ignoreSSL || sp.IgnoreSSL
			return runProbe(cmd, sp.URL, flags)
		},
	}
	bindProbeFlags(cmd, f)
	return cmd
}

func newSavedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "saved",
		Short: "Manage named probe templates (library)",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List saved probe templates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			repo, err := openHistory()
			if err != nil {
				return err
			}
			defer repo.Close()
			list, err := repo.ListSaved(context.Background())
			if err != nil {
				return err
			}
			if len(list) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no saved probes yet — use: hoptrace save <name> <url>")
				return nil
			}
			for _, sp := range list {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s  %-6s  %s\n", sp.Name, sp.Method, sp.URL)
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "show [name]",
		Short: "Show a saved probe template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := openHistory()
			if err != nil {
				return err
			}
			defer repo.Close()
			sp, err := repo.GetSaved(context.Background(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "name=%s method=%s follow=%v\nurl=%s\n", sp.Name, sp.Method, sp.FollowRedirects, sp.URL)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "delete [name]",
		Short: "Delete a saved probe template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := openHistory()
			if err != nil {
				return err
			}
			defer repo.Close()
			if err := repo.DeleteSaved(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", args[0])
			return nil
		},
	})
	return cmd
}

func openHistory() (*repository.SQLiteRepository, error) {
	path, err := repository.DefaultDBPath()
	if err != nil {
		return nil, err
	}
	return repository.OpenSQLite(path)
}

func usage(format string, args ...any) error {
	err := platform.NewUsageError(format, args...)
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(platform.ExitUsage)
	return err
}

func readData(data string) ([]byte, error) {
	if strings.HasPrefix(data, "@") {
		path := strings.TrimPrefix(data, "@")
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read data file: %w", err)
		}
		return b, nil
	}
	return []byte(data), nil
}
