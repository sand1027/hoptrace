package probe

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// Analyzer is the Facade over DNS, executor, SLO, export, and redirect flow.
type Analyzer struct {
	DNS      DNSResolver
	Executor RequestExecutor
	SLO      *SLOEvaluator
	Listener PhaseListener
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

// NewAnalyzer constructs a configured Analyzer (Factory).
func NewAnalyzer(opts ...Option) *Analyzer {
	a := &Analyzer{
		SLO:      NewSLOEvaluator(),
		Listener: NoopListener{},
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Analyze runs the full probe including optional redirect following (Template Method skeleton).
func (a *Analyzer) Analyze(ctx context.Context, req ProbeRequest) (ProbeReport, error) {
	if a.Executor == nil {
		return ProbeReport{}, fmt.Errorf("analyzer: executor is required")
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
		stepReq := req
		stepReq.Method = method
		stepReq.Body = body
		stepReq.URL = currentURL

		step, err := a.doStep(ctx, stepReq, currentURL, hop)
		report.Steps = append(report.Steps, step)
		if err != nil {
			lastErr = err
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
	return report, lastErr
}

func (a *Analyzer) doStep(ctx context.Context, req ProbeRequest, targetURL string, stepNum int) (StepResult, error) {
	step, err := a.Executor.Execute(ctx, req, targetURL)
	step.StepNumber = stepNum
	a.Listener.OnPhase("total", stepNum, step.Timing.TotalMS)
	a.Listener.OnPhase("dns", stepNum, step.Timing.DNSMS)
	a.Listener.OnPhase("connect", stepNum, step.Timing.ConnectMS)
	a.Listener.OnPhase("tls", stepNum, step.Timing.TLSMS)
	a.Listener.OnPhase("ttfb", stepNum, step.Timing.TTFBMS)
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
