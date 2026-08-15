package export

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/sandeepv/hoptrace/internal/probe"
)

type phaseRow struct {
	label string
	ms    float64
	color lipgloss.Color
}

func useColor(w io.Writer) bool {
	// Explicit hoptrace opt-in wins (useful when parent env sets NO_COLOR).
	if os.Getenv("HOPTRACE_COLOR") == "1" || os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

func (e *WaterfallExporter) Export(report probe.ProbeReport) error {
	if useColor(e.W) {
		return e.exportRich(report)
	}
	return e.exportPlain(report)
}

func (e *WaterfallExporter) exportRich(report probe.ProbeReport) error {
	brand := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("36"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	label := lipgloss.NewStyle().Width(10).Foreground(lipgloss.Color("252"))
	msStyle := lipgloss.NewStyle().Width(10).Align(lipgloss.Right).Foreground(lipgloss.Color("252"))
	pctStyle := lipgloss.NewStyle().Width(5).Align(lipgloss.Right).Foreground(lipgloss.Color("245"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)

	fmt.Fprintln(e.W)
	fmt.Fprintln(e.W, brand.Render("hoptrace")+"  "+muted.Render("HTTP phase profiler"))

	for _, step := range report.Steps {
		fmt.Fprintln(e.W)
		head := fmt.Sprintf("Step %d  %s", step.StepNumber, step.URL)
		fmt.Fprintln(e.W, lipgloss.NewStyle().Bold(true).Render(head))

		if step.Error != nil {
			fmt.Fprintln(e.W, errStyle.Render("  ✗  "+*step.Error))
		}

		t := step.Timing
		rows := []phaseRow{
			{"DNS", t.DNSMS, "39"},
			{"Connect", t.ConnectMS, "45"},
			{"TLS", t.TLSMS, "141"},
			{"Wait", t.WaitMS, "215"},
			{"Transfer", t.XferMS, "42"},
		}

		fmt.Fprintln(e.W, muted.Render(fmt.Sprintf("  %-10s %10s  %-42s %5s", "Phase", "Time", "Timeline", "%")))
		for _, row := range rows {
			bar := colorBar(row.ms, t.TotalMS, 40, row.color)
			pct := 0.0
			if t.TotalMS > 0 {
				pct = row.ms / t.TotalMS * 100
			}
			fmt.Fprintf(e.W, "  %s %s  %s %s\n",
				label.Render(row.label),
				msStyle.Render(fmt.Sprintf("%.1f ms", row.ms)),
				bar,
				pctStyle.Render(fmt.Sprintf("%.0f%%", pct)),
			)
		}

		fmt.Fprintf(e.W, "  %s %s  %s\n",
			label.Render("Total"),
			msStyle.Render(fmt.Sprintf("%.1f ms", t.TotalMS)),
			muted.Render(fmt.Sprintf("TTFB %.1f ms", t.TTFBMS)),
		)

		meta := []string{}
		if step.Response != nil {
			meta = append(meta, statusBadge(step.Response.Status))
			meta = append(meta, muted.Render(fmt.Sprintf("%s", humanBytes(step.Response.Bytes))))
		}
		if step.Network.IP != "" {
			meta = append(meta, muted.Render(fmt.Sprintf("%s (%s)", step.Network.IP, step.Network.IPFamily)))
		}
		if step.Network.TLSVersion != "" {
			meta = append(meta, muted.Render(step.Network.TLSVersion))
			if step.Network.TLSCipher != "" {
				meta = append(meta, muted.Render(shortCipher(step.Network.TLSCipher)))
			}
		}
		if step.Network.HTTPVersion != "" {
			meta = append(meta, muted.Render(step.Network.HTTPVersion))
		}
		if step.Timing.IsEstimated {
			meta = append(meta, muted.Render("estimated"))
		}
		if len(meta) > 0 {
			fmt.Fprintln(e.W, "  "+strings.Join(meta, muted.Render(" · ")))
		}
		if line := formatCertLine(step.Network); line != "" {
			daysStyle := muted
			if step.Network.CertDaysLeft != nil && *step.Network.CertDaysLeft <= 30 {
				daysStyle = warnStyle
			}
			fmt.Fprintln(e.W, "  "+daysStyle.Render(line))
		}
	}

	fmt.Fprintln(e.W)
	sumLine := fmt.Sprintf("%d step(s) · %.1f ms · status %d · %s",
		report.TotalSteps, report.Summary.TotalTimeMS, report.Summary.FinalStatus, report.Summary.FinalURL)
	if report.Summary.Errors > 0 {
		fmt.Fprintln(e.W, errStyle.Render("✗  "+sumLine))
	} else {
		fmt.Fprintln(e.W, okStyle.Render("✓  "+sumLine))
	}

	if report.Summary.SLO != nil {
		if report.Summary.SLO.Pass {
			fmt.Fprintln(e.W, okStyle.Render("✓  SLO pass"))
		} else {
			fmt.Fprintln(e.W, errStyle.Render("✗  SLO fail"))
			for _, v := range report.Summary.SLO.Violations {
				fmt.Fprintln(e.W, warnStyle.Render(fmt.Sprintf(
					"   %s  actual %.1f · limit %.1f · Δ +%.1f ms",
					v.Key, v.ActualMS, v.ThresholdMS, v.DeltaMS,
				)))
			}
		}
	}
	fmt.Fprintln(e.W)
	return nil
}

func (e *WaterfallExporter) exportPlain(report probe.ProbeReport) error {
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
		if line := formatCertLine(step.Network); line != "" {
			fmt.Fprintf(e.W, "  %s\n", line)
		}
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

func colorBar(ms, total float64, width int, color lipgloss.Color) string {
	n := 0
	if total > 0 {
		n = int(math.Round(ms / total * float64(width)))
	}
	if n > width {
		n = width
	}
	if ms > 0 && n == 0 {
		n = 1
	}
	filled := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", n))
	empty := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render(strings.Repeat("░", width-n))
	return filled + empty
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

func statusBadge(code int) string {
	text := fmt.Sprintf("%d", code)
	switch {
	case code >= 200 && code < 300:
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")).Render(text)
	case code >= 300 && code < 400:
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render(text)
	case code >= 400:
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203")).Render(text)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(text)
	}
}

func humanBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}

func shortCipher(c string) string {
	c = strings.TrimPrefix(c, "TLS_")
	if len(c) > 36 {
		return c[:36] + "…"
	}
	return c
}

func formatCertLine(n probe.Network) string {
	if n.CertCN == "" && n.CertDaysLeft == nil {
		return ""
	}
	parts := []string{"cert"}
	if n.CertCN != "" {
		parts = append(parts, n.CertCN)
	}
	if n.CertDaysLeft != nil {
		parts = append(parts, fmt.Sprintf("%dd left", *n.CertDaysLeft))
	}
	if n.TLSCustomCA != nil && *n.TLSCustomCA {
		parts = append(parts, "custom CA")
	}
	return strings.Join(parts, " · ")
}
