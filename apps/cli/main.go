package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sandeepv/hoptrace/internal/export"
	"github.com/sandeepv/hoptrace/internal/platform"
	"github.com/sandeepv/hoptrace/internal/probe"
	"github.com/sandeepv/hoptrace/internal/setup"
	"github.com/sandeepv/hoptrace/internal/slo"
)

func main() {
	var (
		method      string
		headers     []string
		data        string
		follow      bool
		timeoutSec  float64
		proxy       string
		ignoreSSL   bool
		cacert      string
		compact     bool
		metricsOnly bool
		jsonPath    string
		sloSpec     string
	)

	root := &cobra.Command{
		Use:   "hoptrace [url]",
		Short: "HTTP latency profiler — DNS, connect, TLS, wait, transfer",
		Long: `hoptrace probes a URL and breaks timing into DNS, connect, TLS, wait, and transfer.

Examples:
  hoptrace https://example.com
  hoptrace --follow 'https://example.com/path?a=1&b=2'
  hoptrace --metrics-only https://httpbin.io/get
  hoptrace -X POST -d '{"ok":true}' https://httpbin.io/post`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProbe(cmd, args[0], probeFlags{
				method:      method,
				headers:     headers,
				data:        data,
				follow:      follow,
				timeoutSec:  timeoutSec,
				proxy:       proxy,
				proxySet:    cmd.Flags().Changed("proxy"),
				ignoreSSL:   ignoreSSL,
				cacert:      cacert,
				compact:     compact,
				metricsOnly: metricsOnly,
				jsonPath:    jsonPath,
				sloSpec:     sloSpec,
			})
		},
	}

	root.Flags().StringVarP(&method, "method", "X", "", "HTTP method")
	root.Flags().StringArrayVarP(&headers, "header", "H", nil, "Request header (repeatable)")
	root.Flags().StringVarP(&data, "data", "d", "", "Request body (or @file)")
	root.Flags().BoolVarP(&follow, "follow", "L", false, "Follow redirects")
	root.Flags().Float64VarP(&timeoutSec, "timeout", "m", 30, "Timeout in seconds")
	root.Flags().StringVarP(&proxy, "proxy", "x", "", "Proxy URL (empty disables env proxy)")
	root.Flags().BoolVarP(&ignoreSSL, "ignore-ssl", "k", false, "Skip TLS verification")
	root.Flags().StringVar(&cacert, "cacert", "", "Custom CA bundle path")
	root.Flags().BoolVar(&compact, "compact", false, "Compact one-line output")
	root.Flags().BoolVar(&metricsOnly, "metrics-only", false, "Metrics-only scripting output")
	root.Flags().StringVar(&jsonPath, "json", "", "Write JSON report to path")
	root.Flags().StringVar(&sloSpec, "slo", "", "SLO thresholds e.g. total=500,ttfb=200")

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "hoptrace v1.0.0")
		},
	})

	// Keep `hoptrace probe <url>` as an alias for muscle memory / scripts.
	probeCmd := &cobra.Command{
		Use:   "probe [url]",
		Short: "Alias for probing a URL (same as hoptrace <url>)",
		Args:  cobra.ExactArgs(1),
		RunE:  root.RunE,
	}
	probeCmd.Flags().AddFlagSet(root.Flags())
	root.AddCommand(probeCmd)

	if err := root.Execute(); err != nil {
		os.Exit(platform.ExitSoftware)
	}
}

type probeFlags struct {
	method, data, proxy, cacert, jsonPath, sloSpec string
	headers                                        []string
	follow, ignoreSSL, compact, metricsOnly        bool
	timeoutSec                                     float64
	proxySet                                       bool
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

	if probeErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "error: %v\n", probeErr)
		os.Exit(platform.ExitTempFail)
	}
	if report.Summary.SLO != nil && !report.Summary.SLO.Pass {
		os.Exit(platform.ExitSLOFail)
	}
	return nil
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
