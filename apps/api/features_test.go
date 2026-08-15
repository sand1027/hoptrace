package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandeepv/hoptrace/internal/ratelimit"
	"github.com/sandeepv/hoptrace/internal/repository"
	"github.com/sandeepv/hoptrace/internal/ssrf"
)

func TestAPI_HealthAndProbeHappyPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api.db")
	repo, err := repository.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(upstream.Close)

	s := &server{
		repo:    repo,
		guard:   ssrf.NewGuard(true), // allow httptest loopback
		logger:  silentLogger{},
		limiter: ratelimit.New(100, 200),
	}
	h := s.routes(false, path)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/health", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("health=%d", rr.Code)
	}

	body, _ := json.Marshal(map[string]any{"url": upstream.URL, "follow_redirects": false})
	req := httptest.NewRequest(http.MethodPost, "/v1/probes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req)
	if rr2.Code != http.StatusOK {
		t.Fatalf("probe=%d body=%s", rr2.Code, rr2.Body.String())
	}
	if !strings.Contains(rr2.Body.String(), `"final_status":200`) && !strings.Contains(rr2.Body.String(), `"FinalStatus"`) {
		// JSON uses final_status in summary
		if !strings.Contains(rr2.Body.String(), "200") {
			t.Fatalf("probe body=%s", rr2.Body.String())
		}
	}
}

func TestAPI_SavedProbeCRUD(t *testing.T) {
	path := filepath.Join(t.TempDir(), "saved.db")
	repo, err := repository.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	s := &server{repo: repo, guard: ssrf.NewGuard(true), logger: silentLogger{}, limiter: ratelimit.New(50, 100)}
	h := s.routes(false, path)

	payload, _ := json.Marshal(map[string]any{
		"name": "demo", "url": "https://example.com/", "method": "GET",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/saved", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("upsert saved=%d %s", rr.Code, rr.Body.String())
	}

	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/v1/saved", nil))
	if rr2.Code != http.StatusOK || !strings.Contains(rr2.Body.String(), "demo") {
		t.Fatalf("list saved=%d %s", rr2.Code, rr2.Body.String())
	}

	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, httptest.NewRequest(http.MethodDelete, "/v1/saved/demo", nil))
	if rr3.Code != http.StatusOK {
		t.Fatalf("delete=%d %s", rr3.Code, rr3.Body.String())
	}
}

type silentLogger struct{}

func (silentLogger) Info(string, ...any)  {}
func (silentLogger) Error(string, ...any) {}
func (silentLogger) Warn(string, ...any)  {}
