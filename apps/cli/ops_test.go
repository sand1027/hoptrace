package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sandeepv/hoptrace/internal/schedule"
)

func mockAPI(t *testing.T) *httptest.Server {
	t.Helper()
	jobs := []schedule.Job{}
	keys := []map[string]any{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/v1/schedules", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(jobs)
		case http.MethodPost:
			var job schedule.Job
			_ = json.NewDecoder(r.Body).Decode(&job)
			job.ID = "sch-test-1"
			jobs = append(jobs, job)
			_ = json.NewEncoder(w).Encode(job)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/v1/schedules/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	})
	mux.HandleFunc("/v1/keys", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(keys)
		case http.MethodPost:
			rec := map[string]any{
				"id": "key-1", "prefix": "ht_abc", "key": "ht_plaintext", "workspace": "default",
			}
			keys = append(keys, map[string]any{
				"id": "key-1", "prefix": "ht_abc", "workspace_id": "default", "name": "ci",
			})
			_ = json.NewEncoder(w).Encode(rec)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/v1/baselines", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(schedule.BaselineStats{
			URL: r.URL.Query().Get("url"), Count: 3, P50MS: 10, P95MS: 20, MeanMS: 12, LastMS: 11,
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("HOPTRACE_API", srv.URL)
	return srv
}

func TestCLI_ScheduleCommands(t *testing.T) {
	mockAPI(t)
	out, _, err := runCLI(t, "schedule", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "no schedules") {
		t.Fatalf("list empty: %q", out)
	}

	out, _, err = runCLI(t, "schedule", "add", "uptime", "https://example.com/", "--every", "30")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "scheduled") {
		t.Fatalf("add: %q", out)
	}

	out, _, err = runCLI(t, "schedule", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "uptime") {
		t.Fatalf("list after add: %q", out)
	}

	out, _, err = runCLI(t, "schedule", "delete", "sch-test-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "deleted") {
		t.Fatalf("delete: %q", out)
	}
}

func TestCLI_KeysCommands(t *testing.T) {
	mockAPI(t)
	out, _, err := runCLI(t, "keys", "create", "ci")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ht_plaintext") {
		t.Fatalf("create: %q", out)
	}

	out, _, err = runCLI(t, "keys", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "key-1") {
		t.Fatalf("list: %q", out)
	}
}

func TestCLI_Baseline(t *testing.T) {
	mockAPI(t)
	out, _, err := runCLI(t, "baseline", "https://example.com/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "p50=") || !strings.Contains(out, "count=3") {
		t.Fatalf("baseline: %q", out)
	}
}

func TestCLI_APIKeyHeader(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		_ = json.NewEncoder(w).Encode([]schedule.Job{})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("HOPTRACE_API", srv.URL)
	t.Setenv("HOPTRACE_API_KEY", "ht_secret")
	_, _, err := runCLI(t, "schedule", "list")
	if err != nil {
		t.Fatal(err)
	}
	if gotKey != "ht_secret" {
		t.Fatalf("api key header=%q", gotKey)
	}
}
