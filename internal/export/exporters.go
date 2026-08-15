package export

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sandeepv/hoptrace/internal/plugin"
	"github.com/sandeepv/hoptrace/internal/probe"
)

// Factory creates an Exporter from a mode name (Factory pattern + plugin registry).
func Factory(mode string, w io.Writer, jsonPath string) (probe.Exporter, error) {
	if w == nil {
		w = os.Stdout
	}
	mode = strings.ToLower(mode)
	if f, ok := plugin.LookupExporter(mode); ok {
		return f(), nil
	}
	switch mode {
	case "", "waterfall", "rich":
		return &WaterfallExporter{W: w}, nil
	case "compact":
		return &CompactExporter{W: w}, nil
	case "metrics", "metrics-only":
		return &MetricsExporter{W: w}, nil
	case "json":
		return &JSONExporter{Path: jsonPath, W: w}, nil
	case "jsonl":
		return &JSONLExporter{W: w}, nil
	case "noop":
		return probe.NoopExporter{}, nil
	default:
		return nil, fmt.Errorf("unknown export mode: %s (registered: %v)", mode, plugin.ListExporters())
	}
}

// WaterfallExporter prints a phase table (Strategy).
type WaterfallExporter struct {
	W io.Writer
}

func (e *WaterfallExporter) Export(report probe.ProbeReport) error {
	for _, step := range report.Steps {
		fmt.Fprintf(e.W, "\n── Step %d: %s ──\n", step.StepNumber, step.URL)
		if step.Error != nil {
			fmt.Fprintf(e.W, "  error: %s\n", *step.Error)
		}
		t := step.Timing
		printBar(e.W, "DNS", t.DNSMS, t.TotalMS)
		printBar(e.W, "Connect", t.ConnectMS, t.TotalMS)
		printBar(e.W, "TLS", t.TLSMS, t.TotalMS)
		printBar(e.W, "Wait", t.WaitMS, t.TotalMS)
		printBar(e.W, "Transfer", t.XferMS, t.TotalMS)
		fmt.Fprintf(e.W, "  %-10s %8.1f ms  (ttfb=%.1f)\n", "Total", t.TotalMS, t.TTFBMS)
		if step.Response != nil {
			fmt.Fprintf(e.W, "  status=%d bytes=%d", step.Response.Status, step.Response.Bytes)
		}
		if step.Network.IP != "" {
			fmt.Fprintf(e.W, " ip=%s (%s)", step.Network.IP, step.Network.IPFamily)
		}
		if step.Network.TLSVersion != "" {
			fmt.Fprintf(e.W, " %s %s", step.Network.TLSVersion, step.Network.TLSCipher)
		}
		fmt.Fprintln(e.W)
	}
	fmt.Fprintf(e.W, "\nSummary: steps=%d total=%.1f ms final_status=%d final_url=%s\n",
		report.TotalSteps, report.Summary.TotalTimeMS, report.Summary.FinalStatus, report.Summary.FinalURL)
	if report.Summary.SLO != nil {
		if report.Summary.SLO.Pass {
			fmt.Fprintln(e.W, "SLO: pass")
		} else {
			fmt.Fprintln(e.W, "SLO: FAIL")
			for _, v := range report.Summary.SLO.Violations {
				fmt.Fprintf(e.W, "  %s: actual=%.1f threshold=%.1f delta=+%.1f\n",
					v.Key, v.ActualMS, v.ThresholdMS, v.DeltaMS)
			}
		}
	}
	return nil
}

func printBar(w io.Writer, label string, ms, total float64) {
	width := 40
	n := 0
	if total > 0 {
		n = int(ms / total * float64(width))
	}
	if n > width {
		n = width
	}
	if ms > 0 && n == 0 {
		n = 1
	}
	bar := strings.Repeat("█", n) + strings.Repeat("░", width-n)
	fmt.Fprintf(w, "  %-10s %8.1f ms  |%s|\n", label, ms, bar)
}

// CompactExporter prints one line per step.
type CompactExporter struct{ W io.Writer }

func (e *CompactExporter) Export(report probe.ProbeReport) error {
	for _, s := range report.Steps {
		status := 0
		bytes := int64(0)
		if s.Response != nil {
			status = s.Response.Status
			bytes = s.Response.Bytes
		}
		fmt.Fprintf(e.W, "step=%d status=%d total=%.1fms ttfb=%.1fms dns=%.1f connect=%.1f tls=%.1f bytes=%d url=%s\n",
			s.StepNumber, status, s.Timing.TotalMS, s.Timing.TTFBMS, s.Timing.DNSMS, s.Timing.ConnectMS, s.Timing.TLSMS, bytes, s.URL)
	}
	return nil
}

// MetricsExporter prints scripting-friendly metrics lines.
type MetricsExporter struct{ W io.Writer }

func (e *MetricsExporter) Export(report probe.ProbeReport) error {
	for _, s := range report.Steps {
		status := 0
		bytes := int64(0)
		if s.Response != nil {
			status = s.Response.Status
			bytes = s.Response.Bytes
		}
		proxy := "direct"
		if s.Network.ProxyURL != nil && *s.Network.ProxyURL != "" {
			proxy = *s.Network.ProxyURL
		}
		line := fmt.Sprintf(
			"Step %d: dns=%.1f connect=%.1f tls=%.1f ttfb=%.1f wait=%.1f xfer=%.1f total=%.1f status=%d bytes=%d ip=%s family=%s tls_version=%s proxy=%s",
			s.StepNumber, s.Timing.DNSMS, s.Timing.ConnectMS, s.Timing.TLSMS, s.Timing.TTFBMS,
			s.Timing.WaitMS, s.Timing.XferMS, s.Timing.TotalMS, status, bytes,
			s.Network.IP, s.Network.IPFamily, s.Network.TLSVersion, proxy,
		)
		if report.Summary.SLO != nil && s.StepNumber == report.TotalSteps {
			if report.Summary.SLO.Pass {
				line += " slo=pass"
			} else {
				keys := make([]string, 0, len(report.Summary.SLO.Violations))
				for _, v := range report.Summary.SLO.Violations {
					keys = append(keys, v.Key)
				}
				line += " slo=fail slo_violations=" + strings.Join(keys, ",")
			}
		}
		fmt.Fprintln(e.W, line)
	}
	return nil
}

// JSONExporter writes the full report as JSON.
type JSONExporter struct {
	Path string
	W    io.Writer
}

func (e *JSONExporter) Export(report probe.ProbeReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if e.Path != "" {
		return os.WriteFile(e.Path, data, 0o644)
	}
	_, err = e.W.Write(append(data, '\n'))
	return err
}
