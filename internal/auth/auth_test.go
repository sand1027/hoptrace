package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sandeepv/hoptrace/internal/auth"
)

type memStore struct {
	byHash map[string]auth.KeyRecord
}

func (m *memStore) FindByHash(_ context.Context, hash string) (auth.KeyRecord, error) {
	rec, ok := m.byHash[hash]
	if !ok {
		return auth.KeyRecord{}, errors.New("not found")
	}
	return rec, nil
}

func (m *memStore) Create(context.Context, string, string, string, string) (auth.KeyRecord, error) {
	return auth.KeyRecord{}, errors.New("unused")
}
func (m *memStore) ListKeys(context.Context) ([]auth.KeyRecord, error) { return nil, nil }
func (m *memStore) DeleteKey(context.Context, string) error            { return nil }
func (m *memStore) Count(context.Context) (int, error) {
	return len(m.byHash), nil
}

func TestMiddleware_SoftAuthAllowsMissingKey(t *testing.T) {
	store := &memStore{byHash: map[string]auth.KeyRecord{}}
	h := auth.Middleware(store, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth.WorkspaceID(r.Context()) != "default" {
			t.Fatalf("workspace=%s", auth.WorkspaceID(r.Context()))
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/probes", nil))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestMiddleware_RequireAuthWithKeys(t *testing.T) {
	plain, _, hash, err := auth.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	store := &memStore{byHash: map[string]auth.KeyRecord{
		hash: {ID: "1", WorkspaceID: "ws_a", KeyHash: hash},
	}}

	okHandler := auth.Middleware(store, true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth.WorkspaceID(r.Context()) != "ws_a" {
			t.Fatalf("workspace=%s", auth.WorkspaceID(r.Context()))
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	// missing key → 401 when keys exist + require-auth
	rr := httptest.NewRecorder()
	okHandler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/probes", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing key status=%d", rr.Code)
	}

	// valid key → ok
	req := httptest.NewRequest(http.MethodGet, "/v1/probes", nil)
	req.Header.Set("X-API-Key", plain)
	rr = httptest.NewRecorder()
	okHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("valid key status=%d body=%s", rr.Code, rr.Body.String())
	}

	// query param (websocket style)
	req = httptest.NewRequest(http.MethodGet, "/v1/probes/stream?api_key="+plain, nil)
	rr = httptest.NewRecorder()
	okHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("query key status=%d", rr.Code)
	}
}
