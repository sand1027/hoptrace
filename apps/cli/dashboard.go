package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

func dashboardURL() string {
	if v := os.Getenv("HOPTRACE_DASHBOARD"); v != "" {
		return v
	}
	return "http://127.0.0.1:3000"
}

func newDashboardCmd() *cobra.Command {
	var (
		urlFlag string
		noOpen  bool
	)
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Open the hoptrace web dashboard in your browser",
		Long: `Open the local hoptrace dashboard (send probes, waterfall, history).

Defaults to http://127.0.0.1:3000 (override with --url or HOPTRACE_DASHBOARD).

Start the stack first if needed:
  make dev          # API :8080 + web :3000
  # or: make api & make web`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			url := urlFlag
			if url == "" {
				url = dashboardURL()
			}

			apiOK := pingURL(apiBase()+"/v1/health", 800*time.Millisecond)
			dashOK := pingURL(url, 800*time.Millisecond)

			if !apiOK {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: API not reachable at %s — run: make api\n", apiBase())
			}
			if !dashOK {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: dashboard not reachable at %s — run: make web  (or make dev)\n", url)
			}

			fmt.Fprintln(cmd.OutOrStdout(), url)
			if noOpen {
				return nil
			}
			if err := openBrowser(url); err != nil {
				return fmt.Errorf("open browser: %w (open %s manually)", err, url)
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "opened in browser")
			return nil
		},
	}
	cmd.Flags().StringVar(&urlFlag, "url", "", "Dashboard URL (default HOPTRACE_DASHBOARD or http://127.0.0.1:3000)")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Print the URL only; do not open a browser")
	return cmd
}

func pingURL(raw string, timeout time.Duration) bool {
	client := &http.Client{Timeout: timeout}
	res, err := client.Get(raw)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	return res.StatusCode > 0 && res.StatusCode < 500
}

func openBrowser(url string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{url}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		name, args = "xdg-open", []string{url}
	}
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}
