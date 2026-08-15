package setup

import (
	"github.com/sandeepv/hoptrace/internal/dns"
	"github.com/sandeepv/hoptrace/internal/executor"
	"github.com/sandeepv/hoptrace/internal/platform"
	"github.com/sandeepv/hoptrace/internal/probe"
)

// DefaultAnalyzer wires production strategies (Factory convenience).
// Lives outside probe/ to avoid import cycles with executor.
func DefaultAnalyzer(opts ...probe.Option) *probe.Analyzer {
	inner := executor.NewHTTPExecutor()
	decorated := executor.NewLoggingExecutor(inner, platform.Logger())
	base := []probe.Option{
		probe.WithDNS(dns.NewSystemResolver()),
		probe.WithExecutor(decorated),
	}
	base = append(base, opts...)
	return probe.NewAnalyzer(base...)
}

// AnalyzerWithGuard is DefaultAnalyzer plus an SSRF URL validator (API path).
func AnalyzerWithGuard(v probe.URLValidator, opts ...probe.Option) *probe.Analyzer {
	opts = append([]probe.Option{probe.WithURLValidator(v)}, opts...)
	return DefaultAnalyzer(opts...)
}
