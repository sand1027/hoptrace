package plugin

import (
	"fmt"
	"sync"

	"github.com/sandeepv/hoptrace/internal/probe"
)

// ExporterFactory creates an exporter (extension point).
type ExporterFactory func() probe.Exporter

var (
	mu        sync.RWMutex
	exporters = map[string]ExporterFactory{}
)

// RegisterExporter adds a named exporter factory (Open/Closed via registry).
func RegisterExporter(name string, f ExporterFactory) {
	mu.Lock()
	defer mu.Unlock()
	exporters[name] = f
}

// LookupExporter returns a registered factory.
func LookupExporter(name string) (ExporterFactory, bool) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := exporters[name]
	return f, ok
}

// ListExporters returns registered names.
func ListExporters() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(exporters))
	for k := range exporters {
		out = append(out, k)
	}
	return out
}

// MustRegister panics on duplicate — for init() in plugins.
func MustRegister(name string, f ExporterFactory) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := exporters[name]; ok {
		panic(fmt.Sprintf("exporter already registered: %s", name))
	}
	exporters[name] = f
}
