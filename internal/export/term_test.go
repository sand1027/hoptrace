package export

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sandeepv/hoptrace/internal/probe"
)

func sampleReport() probe.ProbeReport {
	days := 143
	return probe.ProbeReport{
		InitialURL: "https://example.com",
		TotalSteps: 1,
		Steps: []probe.StepResult{{
			URL:        "https://example.com",
			StepNumber: 1,
			Timing: probe.Timing{
				DNSMS: 2, ConnectMS: 20, TLSMS: 40, WaitMS: 30, XferMS: 8, TTFBMS: 92, TotalMS: 100,
			},
			Network: probe.Network{
				IP: "93.184.216.34", IPFamily: "IPv4", TLSVersion: "TLSv1.3", TLSCipher: "TLS_AES_128_GCM_SHA256",
				CertCN: "example.com", CertDaysLeft: &days,
			},
			Response: &probe.ResponseMeta{Status: 200, Bytes: 1256},
		}},
		Summary: probe.Summary{
			TotalTimeMS: 100, FinalStatus: 200, FinalURL: "https://example.com", FinalBytes: 1256,
		},
	}
}

func TestWaterfallPlainContainsPhases(t *testing.T) {
	var buf bytes.Buffer
	exp := &WaterfallExporter{W: &buf}
	if err := exp.Export(sampleReport()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"DNS", "Connect", "TLS", "Wait", "Transfer", "Total", "200", "cert · example.com · 143d left"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestWaterfallForceColorRich(t *testing.T) {
	t.Setenv("FORCE_COLOR", "1")
	t.Setenv("NO_COLOR", "")
	var buf bytes.Buffer
	exp := &WaterfallExporter{W: &buf}
	if err := exp.Export(sampleReport()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "hoptrace") {
		t.Fatalf("expected rich header, got:\n%s", out)
	}
	if !strings.Contains(out, "Timeline") {
		t.Fatalf("expected timeline header, got:\n%s", out)
	}
}
