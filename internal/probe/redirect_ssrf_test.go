package probe_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/sandeepv/hoptrace/internal/probe"
)

type stubExec struct {
	calls int
	locs  []string
}

func (s *stubExec) Execute(_ context.Context, _ probe.ProbeRequest, target string) (probe.StepResult, error) {
	s.calls++
	loc := ""
	if s.calls-1 < len(s.locs) {
		loc = s.locs[s.calls-1]
	}
	status := 200
	var locPtr *string
	if loc != "" {
		status = 302
		locPtr = &loc
	}
	return probe.StepResult{
		URL: target,
		Response: &probe.ResponseMeta{
			Status:   status,
			Location: locPtr,
		},
	}, nil
}

type blockingValidator struct{}

func (blockingValidator) ValidateURL(raw string) error {
	if raw == "http://127.0.0.1/secret" {
		return fmt.Errorf("ssrf: blocked")
	}
	return nil
}

func TestAnalyzer_ValidatesRedirectHops(t *testing.T) {
	ex := &stubExec{locs: []string{"http://127.0.0.1/secret"}}
	a := probe.NewAnalyzer(
		probe.WithExecutor(ex),
		probe.WithURLValidator(blockingValidator{}),
	)
	req, err := probe.NewRequestBuilder().
		URL("https://public.example/start").
		FollowRedirect(true).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	report, err := a.Analyze(context.Background(), req)
	if err == nil {
		t.Fatal("expected ssrf error on redirect hop")
	}
	if ex.calls != 1 {
		t.Fatalf("executor should stop after first hop, calls=%d", ex.calls)
	}
	if len(report.Steps) != 2 {
		t.Fatalf("want start + blocked hop steps, got %d", len(report.Steps))
	}
}
