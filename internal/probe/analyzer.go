package probe

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// Analyzer is the Facade over DNS, executor, SLO, export, and redirect flow.
type Analyzer struct {
	DNS       DNSResolver
	Executor  RequestExecutor
	SLO       *SLOEvaluator
	Events    EventListener
	Validator URLValidator // optional; re-checked on every redirect hop
	// Listener is legacy; prefer Events. Kept for WithListener.
	Listener PhaseListener
}

// URLValidator blocks unsafe targets (SSRF) — Strategy.
type URLValidator interface {
	ValidateURL(raw string) error
}

// Option configures an Analyzer (functional options + Factory).
type Option func(*Analyzer)

func WithDNS(d DNSResolver) Option {
	return func(a *Analyzer) { a.DNS = d }
}

func WithExecutor(e RequestExecutor) Option {
	return func(a *Analyzer) { a.Executor = e }
}

func WithListener(l PhaseListener) Option {
	return func(a *Analyzer) { a.Listener = l }
}

func WithEvents(l EventListener) Option {
	return func(a *Analyzer) { a.Events = l }
}

func WithURLValidator(v URLValidator) Option {
	return func(a *Analyzer) { a.Validator = v }
}

// NewAnalyzer constructs a configured Analyzer (Factory).
func NewAnalyzer(opts ...Option) *Analyzer {
	a := &Analyzer{
		SLO:      NewSLOEvaluator(),
		Listener: NoopListener{},
		Events:   NoopEventListener{},
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *Analyzer) emit(e Event) {
	listeners := []EventListener{a.Events}
	if a.Listener != nil {
		listeners = append(listeners, PhaseBridge{Inner: a.Listener})
	}
	NewMultiListener(listeners...).OnEvent(e)
}

// Analyze runs the full probe including optional redirect following (Template Method skeleton).
func (a *Analyzer) Analyze(ctx context.Context, req ProbeRequest) (ProbeReport, error) {
	if a.Executor == nil {
		return ProbeReport{}, fmt.Errorf("analyzer: executor is required")
	}
	if a.Events == nil {
		a.Events = NoopEventListener{}
	}
	if a.Listener == nil {
		a.Listener = NoopListener{}
	}

	report := ProbeReport{
		InitialURL: req.URL,
		Steps:      []StepResult{},
	}

	currentURL := req.URL
	method := req.Method
	body := req.Body
	maxHops := 1
	if req.FollowRedirect {
		maxHops = req.MaxRedirects
		if maxHops <= 0 {
			maxHops = 10
		}
	}

	var lastErr error
	for hop := 1; hop <= maxHops; hop++ {
		if a.Validator != nil {
			if err := a.Validator.ValidateURL(currentURL); err != nil {
				msg := err.Error()
				step := StepResult{URL: currentURL, StepNumber: hop, Error: &msg}
				report.Steps = append(report.Steps, step)
				lastErr = err
				a.emit(Event{Kind: "error", Step: hop, URL: currentURL, Error: msg})
				break
			}
		}

		stepReq := req
		stepReq.Method = method
		stepReq.Body = body
		stepReq.URL = currentURL

		a.emit(Event{Kind: "step_start", Step: hop, URL: currentURL})
		step, err := a.doStep(ctx, stepReq, currentURL, hop)
		report.Steps = append(report.Steps, step)
		a.emit(Event{Kind: "step_done", Step: hop, URL: currentURL, Result: &step})
		if err != nil {
			lastErr = err
			a.emit(Event{Kind: "error", Step: hop, URL: currentURL, Error: err.Error()})
			break
		}

		next, shouldFollow := a.nextRedirectURL(step, req.FollowRedirect)
		if !shouldFollow {
			break
		}
		currentURL = next
		if step.Response != nil && step.Response.Status >= 301 && step.Response.Status <= 303 {
			method = "GET"
			body = nil
		}
		if hop == maxHops {
			note := "max redirects reached"
			step.Note = &note
			report.Steps[len(report.Steps)-1] = step
		}
	}

	report.TotalSteps = len(report.Steps)
	report.Summary = a.buildSummary(report, req.SLO)
	a.emit(Event{Kind: "done", Report: &report, Error: errString(lastErr)})
	return report, lastErr
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (a *Analyzer) doStep(ctx context.Context, req ProbeRequest, targetURL string, stepNum int) (StepResult, error) {
	var preDNSMS float64
	var preIP, preFamily string
	if a.DNS != nil {
		if u, err := url.Parse(targetURL); err == nil && u.Hostname() != "" {
			port := u.Port()
			if port == "" {
				if u.Scheme == "https" {
					port = "443"
				} else {
					port = "80"
				}
			}
			if ip, family, ms, err := a.DNS.Resolve(ctx, u.Hostname(), port); err == nil {
				preDNSMS, preIP, preFamily = ms, ip, family
			}
		}
	}

	step, err := a.Executor.Execute(ctx, req, targetURL)
	step.StepNumber = stepNum
	if step.Timing.DNSMS == 0 && preDNSMS > 0 {
		step.Timing.DNSMS = preDNSMS
	}
	if step.Network.IP == "" && preIP != "" {
		step.Network.IP = preIP
		step.Network.IPFamily = preFamily
	}
	for _, p := range []struct {
		name string
		ms   float64
	}{
		{"dns", step.Timing.DNSMS},
		{"connect", step.Timing.ConnectMS},
		{"tls", step.Timing.TLSMS},
		{"wait", step.Timing.WaitMS},
		{"xfer", step.Timing.XferMS},
		{"ttfb", step.Timing.TTFBMS},
		{"total", step.Timing.TotalMS},
	} {
		a.emit(Event{Kind: "phase", Step: stepNum, Phase: p.name, MS: p.ms, URL: targetURL})
	}
	return step, err
}

func (a *Analyzer) nextRedirectURL(step StepResult, follow bool) (string, bool) {
	if !follow || step.Response == nil {
		return "", false
	}
	status := step.Response.Status
	if status < 300 || status > 399 || step.Response.Location == nil {
		return "", false
	}
	loc := strings.TrimSpace(*step.Response.Location)
	if loc == "" {
		return "", false
	}
	base, err := url.Parse(step.URL)
	if err != nil {
		return "", false
	}
	ref, err := url.Parse(loc)
	if err != nil {
		return "", false
	}
	return base.ResolveReference(ref).String(), true
}

func (a *Analyzer) buildSummary(report ProbeReport, thresholds map[string]float64) Summary {
	sum := Summary{}
	for _, s := range report.Steps {
		sum.TotalTimeMS += s.Timing.TotalMS
		if s.Error != nil {
			sum.Errors++
		}
	}
	if len(report.Steps) > 0 {
		last := report.Steps[len(report.Steps)-1]
		sum.FinalURL = last.URL
		if last.Response != nil {
			sum.FinalStatus = last.Response.Status
			sum.FinalBytes = last.Response.Bytes
		}
		if last.Error == nil && a.SLO != nil {
			sum.SLO = a.SLO.Evaluate(thresholds, last)
		}
	}
	return sum
}
