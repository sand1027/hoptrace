package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/sandeepv/hoptrace/internal/auth"
	"github.com/sandeepv/hoptrace/internal/platform"
	"github.com/sandeepv/hoptrace/internal/ratelimit"
	"github.com/sandeepv/hoptrace/internal/repository"
	"github.com/sandeepv/hoptrace/internal/ssrf"
)

func testAPI(t *testing.T, requireAuth bool, rps float64) http.Handler {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	repo, err := repository.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	s := &server{
		repo:    repo,
		guard:   ssrf.NewGuard(false),
		logger:  platform.Logger(),
		limiter: ratelimit.New(rps, int(rps*2)),
	}
	return s.routes(requireAuth, path)
}

func TestAdversarial_SSRFBlocked(t *testing.T) {
	h := testAPI(t, false, 100)
	body, _ := json.Marshal(map[string]any{
		"url": "http://127.0.0.1/",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/probes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAdversarial_SSRFBlocksWebhookOnSchedule(t *testing.T) {
	h := testAPI(t, false, 100)
	body, _ := json.Marshal(map[string]any{
		"name":          "bad-hook",
		"url":           "https://example.com/",
		"interval_sec":  60,
		"enabled":       false,
		"webhook_url":   "http://169.254.169.254/",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/schedules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAdversarial_RequireAuth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.db")
	repo, err := repository.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	plain, prefix, hash, err := auth.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(t.Context(), "ci", "default", prefix, hash); err != nil {
		t.Fatal(err)
	}
	s := &server{
		repo:    repo,
		guard:   ssrf.NewGuard(false),
		logger:  platform.Logger(),
		limiter: ratelimit.New(100, 200),
	}
	h := s.routes(true, path)

	req := httptest.NewRequest(http.MethodGet, "/v1/probes", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without key, got %d", rr.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/v1/probes", nil)
	req2.Header.Set("X-API-Key", plain)
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 with key, got %d %s", rr2.Code, rr2.Body.String())
	}
}

func TestAdversarial_RateLimit429(t *testing.T) {
	h := testAPI(t, false, 1) // 1 rps, burst 2
	got429 := false
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/probes", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Fatal("expected 429 after bursting rate limit")
	}
}
