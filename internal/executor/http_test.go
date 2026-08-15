package executor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sandeepv/hoptrace/internal/probe"
)

func TestAdaptTimings_FullPhases(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := &TraceTimings{
		DNSStart:     base,
		DNSDone:      base.Add(10 * time.Millisecond),
		ConnectStart: base.Add(10 * time.Millisecond),
		ConnectDone:  base.Add(30 * time.Millisecond),
		TLSStart:     base.Add(30 * time.Millisecond),
		TLSDone:      base.Add(50 * time.Millisecond),
		GotConn:      base.Add(50 * time.Millisecond),
		WroteRequest: base.Add(55 * time.Millisecond),
		GotFirstByte: base.Add(80 * time.Millisecond),
	}
	bodyEnd := base.Add(90 * time.Millisecond)
	got := adaptTimings(tr, 90*time.Millisecond, bodyEnd)

	if got.IsEstimated {
		t.Fatal("expected precise timings")
	}
	assertNear(t, "dns", got.DNSMS, 10)
	assertNear(t, "connect", got.ConnectMS, 20)
	assertNear(t, "tls", got.TLSMS, 20)
	assertNear(t, "wait", got.WaitMS, 25)
	assertNear(t, "xfer", got.XferMS, 10)
	assertNear(t, "ttfb", got.TTFBMS, 80)
	assertNear(t, "total", got.TotalMS, 90)
}

func TestAdaptTimings_MissingFirstByteIsEstimated(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := &TraceTimings{
		DNSStart: base,
		DNSDone:  base.Add(5 * time.Millisecond),
	}
	got := adaptTimings(tr, 40*time.Millisecond, time.Time{})
	if !got.IsEstimated {
		t.Fatal("expected estimated when GotFirstByte missing")
	}
	assertNear(t, "ttfb", got.TTFBMS, 40)
	assertNear(t, "dns", got.DNSMS, 5)
}

func TestAdaptTimings_WaitFallbackEstimated(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := &TraceTimings{
		GotConn:      base,
		GotFirstByte: base.Add(15 * time.Millisecond),
		// no WroteRequest → wait falls back to GotConn delta and marks estimated
	}
	got := adaptTimings(tr, 20*time.Millisecond, base.Add(20*time.Millisecond))
	if !got.IsEstimated {
		t.Fatal("expected estimated wait fallback")
	}
	assertNear(t, "wait", got.WaitMS, 15)
}

func TestHTTPExecutor_LocalServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	ex := NewHTTPExecutor()
	step, err := ex.Execute(context.Background(), probe.ProbeRequest{
		URL:     srv.URL,
		Method:  http.MethodGet,
		Timeout: 5 * time.Second,
		Headers: map[string]string{},
	}, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if step.Response == nil || step.Response.Status != 200 {
		t.Fatalf("response=%+v", step.Response)
	}
	if step.Response.Bytes != 11 {
		t.Fatalf("bytes=%d", step.Response.Bytes)
	}
	if step.Network.HTTPVersion == "" {
		t.Fatal("expected http_version")
	}
	if step.Timing.TotalMS <= 0 {
		t.Fatalf("total=%v", step.Timing.TotalMS)
	}
}

func TestHTTPVersion(t *testing.T) {
	if httpVersion("HTTP/1.1") != "HTTP/1.1" {
		t.Fatal(httpVersion("HTTP/1.1"))
	}
	if httpVersion("HTTP/2.0") != "HTTP/2.0" {
		t.Fatal(httpVersion("HTTP/2.0"))
	}
}

func assertNear(t *testing.T, name string, got, want float64) {
	t.Helper()
	delta := got - want
	if delta < 0 {
		delta = -delta
	}
	if delta > 1.5 { // allow tiny float/jitter slack
		t.Fatalf("%s: got %.3f want ~%.3f", name, got, want)
	}
}
