package executor

import (
	"context"
	"log/slog"

	"github.com/sandeepv/hoptrace/internal/probe"
)

// LoggingExecutor decorates a RequestExecutor with structured logs (Decorator).
type LoggingExecutor struct {
	Inner  probe.RequestExecutor
	Logger *slog.Logger
}

func NewLoggingExecutor(inner probe.RequestExecutor, logger *slog.Logger) *LoggingExecutor {
	return &LoggingExecutor{Inner: inner, Logger: logger}
}

func (d *LoggingExecutor) Execute(ctx context.Context, req probe.ProbeRequest, targetURL string) (probe.StepResult, error) {
	d.Logger.Info("probe.execute.start", "method", req.Method, "url", targetURL)
	step, err := d.Inner.Execute(ctx, req, targetURL)
	if err != nil {
		d.Logger.Warn("probe.execute.error", "url", targetURL, "err", err, "total_ms", step.Timing.TotalMS)
		return step, err
	}
	status := 0
	if step.Response != nil {
		status = step.Response.Status
	}
	d.Logger.Info("probe.execute.done",
		"url", targetURL,
		"status", status,
		"dns_ms", step.Timing.DNSMS,
		"connect_ms", step.Timing.ConnectMS,
		"tls_ms", step.Timing.TLSMS,
		"ttfb_ms", step.Timing.TTFBMS,
		"total_ms", step.Timing.TotalMS,
	)
	return step, nil
}
