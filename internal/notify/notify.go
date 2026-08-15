package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sandeepv/hoptrace/internal/probe"
)

// Event is an alert payload.
type Event struct {
	Title   string            `json:"title"`
	Message string            `json:"message"`
	URL     string            `json:"url"`
	Status  int               `json:"status"`
	TotalMS float64           `json:"total_ms"`
	SLOFail bool              `json:"slo_fail"`
	Meta    map[string]string `json:"meta,omitempty"`
}

// Notifier delivers alerts (Strategy).
type Notifier interface {
	Notify(ctx context.Context, e Event) error
}

// NoopNotifier is a Null Object.
type NoopNotifier struct{}

func (NoopNotifier) Notify(context.Context, Event) error { return nil }

// MultiNotifier fans out to many notifiers.
type MultiNotifier struct {
	Items []Notifier
}

func (m MultiNotifier) Notify(ctx context.Context, e Event) error {
	var first error
	for _, n := range m.Items {
		if n == nil {
			continue
		}
		if err := n.Notify(ctx, e); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// WebhookNotifier POSTs JSON to a URL.
type WebhookNotifier struct {
	URL    string
	Client *http.Client
}

func (w WebhookNotifier) Notify(ctx context.Context, e Event) error {
	if w.URL == "" {
		return nil
	}
	client := w.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	body, _ := json.Marshal(e)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return nil
}

// SlackNotifier posts a simple Slack incoming-webhook message.
type SlackNotifier struct {
	WebhookURL string
	Client     *http.Client
}

func (s SlackNotifier) Notify(ctx context.Context, e Event) error {
	if s.WebhookURL == "" {
		return nil
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	text := fmt.Sprintf("*%s*\n%s\nurl=%s status=%d total=%.1fms slo_fail=%v",
		e.Title, e.Message, e.URL, e.Status, e.TotalMS, e.SLOFail)
	payload, _ := json.Marshal(map[string]string{"text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.WebhookURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack status %d", resp.StatusCode)
	}
	return nil
}

// FromReport builds an Event when SLO failed or hard error.
func FromReport(title string, report probe.ProbeReport, runErr error) (Event, bool) {
	e := Event{
		Title:   title,
		URL:     report.InitialURL,
		Status:  report.Summary.FinalStatus,
		TotalMS: report.Summary.TotalTimeMS,
		Meta:    map[string]string{},
	}
	if runErr != nil {
		e.Message = runErr.Error()
		return e, true
	}
	if report.Summary.SLO != nil && !report.Summary.SLO.Pass {
		e.SLOFail = true
		keys := make([]string, 0, len(report.Summary.SLO.Violations))
		for _, v := range report.Summary.SLO.Violations {
			keys = append(keys, v.Key)
		}
		e.Message = "SLO violations: " + strings.Join(keys, ",")
		return e, true
	}
	return e, false
}
