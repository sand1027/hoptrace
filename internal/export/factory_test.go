package export

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFactoryModes(t *testing.T) {
	var buf bytes.Buffer
	rep := sampleReport()
	for _, mode := range []string{"waterfall", "compact", "metrics-only", "jsonl", "noop"} {
		buf.Reset()
		exp, err := Factory(mode, &buf, "")
		if err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		if err := exp.Export(rep); err != nil {
			t.Fatalf("%s export: %v", mode, err)
		}
		if mode != "noop" && buf.Len() == 0 && mode != "json" {
			// noop writes nothing; others should write
			if mode != "noop" {
				t.Fatalf("%s produced no output", mode)
			}
		}
	}
	path := filepath.Join(t.TempDir(), "out.json")
	exp, err := Factory("json", &buf, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := exp.Export(rep); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "example.com") {
		t.Fatalf("json missing url: %s", b)
	}
	if _, err := Factory("not-a-mode", &buf, ""); err == nil {
		t.Fatal("expected unknown mode error")
	}
}
