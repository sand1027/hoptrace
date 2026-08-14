package probe

import (
	"context"
)

// DNSResolver resolves a host to an IP (Strategy).
type DNSResolver interface {
	Resolve(ctx context.Context, host string, port string) (ip string, family string, dnsMS float64, err error)
}

// TLSInfo holds certificate / handshake details.
type TLSInfo struct {
	Version      string
	Cipher       string
	CertCN       string
	CertDaysLeft *int
	Verified     bool
}

// TLSInspector extracts TLS metadata after connect (Strategy).
type TLSInspector interface {
	Inspect(ctx context.Context, host string, addr string, ignoreSSL bool, caBundle string) (*TLSInfo, float64, error)
}

// RequestExecutor performs a single HTTP hop with timing (Strategy).
type RequestExecutor interface {
	Execute(ctx context.Context, req ProbeRequest, targetURL string) (StepResult, error)
}

// Exporter renders or persists a ProbeReport (Strategy).
type Exporter interface {
	Export(report ProbeReport) error
}

// PhaseListener observes probe lifecycle events (Observer — used more in v4).
type PhaseListener interface {
	OnPhase(phase string, step int, ms float64)
}

// NoopListener is a Null Object for PhaseListener.
type NoopListener struct{}

func (NoopListener) OnPhase(string, int, float64) {}

// NoopExporter is a Null Object for Exporter.
type NoopExporter struct{}

func (NoopExporter) Export(ProbeReport) error { return nil }
