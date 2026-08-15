package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLimiter_BurstThenBlock(t *testing.T) {
	l := New(1, 2)
	if !l.Allow("a") || !l.Allow("a") {
		t.Fatal("burst should allow 2")
	}
	if l.Allow("a") {
		t.Fatal("third should block")
	}
	if !l.Allow("b") {
		t.Fatal("other key independent")
	}
}

func TestMiddleware_Returns429(t *testing.T) {
	l := New(1, 1)
	h := Middleware(l, func(*http.Request) string { return "k" })(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("first=%d", rr.Code)
	}
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("second=%d", rr2.Code)
	}
}
