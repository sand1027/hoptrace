package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sandeepv/hoptrace/internal/probe"
)

func TestFromReport(t *testing.T) {
	_, ok := FromReport("t", probe.ProbeReport{}, nil)
	if ok {
		t.Fatal("ok report should not alert")
	}
	_, ok = FromReport("t", probe.ProbeReport{}, io.EOF)
	if !ok {
		t.Fatal("error should alert")
	}
	rep := probe.ProbeReport{Summary: probe.Summary{
		SLO: &probe.SLOResult{Pass: false, Violations: []probe.SLOViolation{{Key: "total"}}},
	}}
	ev, ok := FromReport("t", rep, nil)
	if !ok || !ev.SLOFail {
		t.Fatalf("slo fail alert: %+v ok=%v", ev, ok)
	}
}

func TestWebhookAndSlack(t *testing.T) {
	var webhookHit, slackHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		if _, hasText := m["text"]; hasText {
			slackHit = true
		} else {
			webhookHit = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	ev := Event{Title: "x", Message: "y", URL: "https://ex", Status: 500}
	n := MultiNotifier{Items: []Notifier{
		WebhookNotifier{URL: srv.URL, Client: srv.Client()},
		SlackNotifier{WebhookURL: srv.URL, Client: srv.Client()},
		NoopNotifier{},
	}}
	if err := n.Notify(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if !webhookHit || !slackHit {
		t.Fatalf("webhook=%v slack=%v", webhookHit, slackHit)
	}
}
