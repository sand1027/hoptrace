package setup

import (
	"github.com/sandeepv/hoptrace/internal/executor"
	"github.com/sandeepv/hoptrace/internal/platform"
	"github.com/sandeepv/hoptrace/internal/probe"
)

// DefaultAnalyzer wires production strategies (Factory convenience).
// Lives outside probe/ to avoid import cycles with executor.
func DefaultAnalyzer() *probe.Analyzer {
	inner := executor.NewHTTPExecutor()
	decorated := executor.NewLoggingExecutor(inner, platform.Logger())
	return probe.NewAnalyzer(probe.WithExecutor(decorated))
}
