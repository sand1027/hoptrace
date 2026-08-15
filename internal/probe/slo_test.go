package probe

import "testing"

func TestSLOEvaluator_PassAndFail(t *testing.T) {
	ev := NewSLOEvaluator()
	step := StepResult{
		Timing: Timing{
			DNSMS: 5, ConnectMS: 10, TLSMS: 20, TTFBMS: 80, WaitMS: 40, XferMS: 5, TotalMS: 90,
		},
	}

	if got := ev.Evaluate(nil, step); got != nil {
		t.Fatalf("empty thresholds should return nil, got %+v", got)
	}

	pass := ev.Evaluate(map[string]float64{"total": 200, "ttfb": 100}, step)
	if pass == nil || !pass.Pass || len(pass.Violations) != 0 {
		t.Fatalf("expected pass, got %+v", pass)
	}

	fail := ev.Evaluate(map[string]float64{"total": 50, "ttfb": 10}, step)
	if fail == nil || fail.Pass {
		t.Fatalf("expected fail, got %+v", fail)
	}
	if len(fail.Violations) != 2 {
		t.Fatalf("expected 2 violations, got %d: %+v", len(fail.Violations), fail.Violations)
	}
}
