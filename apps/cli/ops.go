package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"github.com/sandeepv/hoptrace/internal/schedule"
)

func apiBase() string {
	if v := os.Getenv("HOPTRACE_API"); v != "" {
		return v
	}
	return "http://127.0.0.1:8080"
}

func apiDo(method, path string, body any) (*http.Response, error) {
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, apiBase()+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if k := os.Getenv("HOPTRACE_API_KEY"); k != "" {
		req.Header.Set("X-API-Key", k)
	}
	return http.DefaultClient.Do(req)
}

func newScheduleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Manage scheduled probes (via API)",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List schedules",
		RunE: func(cmd *cobra.Command, _ []string) error {
			resp, err := apiDo(http.MethodGet, "/v1/schedules", nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			var jobs []schedule.Job
			if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
				return err
			}
			if len(jobs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no schedules")
				return nil
			}
			for _, j := range jobs {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  every=%ds enabled=%v last=%d/%.0fms  %s\n",
					j.ID, j.IntervalSec, j.Enabled, j.LastStatus, j.LastTotalMS, j.Name)
			}
			return nil
		},
	})
	var interval int
	var enabled bool
	add := &cobra.Command{
		Use:   "add [name] [url]",
		Short: "Create a schedule",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			job := schedule.Job{
				Name:        args[0],
				URL:         args[1],
				Method:      "GET",
				IntervalSec: interval,
				Enabled:     enabled,
			}
			resp, err := apiDo(http.MethodPost, "/v1/schedules", job)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			var out schedule.Job
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "scheduled %s every %ds\n", out.ID, out.IntervalSec)
			return nil
		},
	}
	add.Flags().IntVar(&interval, "every", 60, "Interval seconds")
	add.Flags().BoolVar(&enabled, "enabled", true, "Enable immediately")
	cmd.AddCommand(add)
	cmd.AddCommand(&cobra.Command{
		Use:   "delete [id]",
		Args:  cobra.ExactArgs(1),
		Short: "Delete a schedule",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiDo(http.MethodDelete, "/v1/schedules/"+args[0], nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			fmt.Fprintln(cmd.OutOrStdout(), "deleted", args[0])
			return nil
		},
	})
	return cmd
}

func newKeysCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "keys", Short: "Manage API keys (via API)"}
	cmd.AddCommand(&cobra.Command{
		Use:   "create [name]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Create an API key (prints plaintext once)",
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "default"
			if len(args) > 0 {
				name = args[0]
			}
			resp, err := apiDo(http.MethodPost, "/v1/keys", map[string]string{"name": name})
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			var out map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "id=%v prefix=%v key=%v\n", out["id"], out["prefix"], out["key"])
			fmt.Fprintln(cmd.ErrOrStderr(), "store HOPTRACE_API_KEY for subsequent API calls")
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List API key metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			resp, err := apiDo(http.MethodGet, "/v1/keys", nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			var list []map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
				return err
			}
			for _, k := range list {
				fmt.Fprintf(cmd.OutOrStdout(), "%v  %v  ws=%v  %v\n", k["id"], k["prefix"], k["workspace_id"], k["name"])
			}
			return nil
		},
	})
	return cmd
}

func newBaselineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "baseline [url]",
		Short: "Show p50/p95 baseline for a URL (via API)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiDo(http.MethodGet, "/v1/baselines?url="+url.QueryEscape(args[0]), nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			var b schedule.BaselineStats
			if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "count=%d p50=%.1f p95=%.1f mean=%.1f last=%.1f regressed=%v %s\n",
				b.Count, b.P50MS, b.P95MS, b.MeanMS, b.LastMS, b.Regressed, b.Message)
			return nil
		},
	}
}

