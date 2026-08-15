package export

import (
	"encoding/json"
	"io"

	"github.com/sandeepv/hoptrace/internal/plugin"
	"github.com/sandeepv/hoptrace/internal/probe"
)

func init() {
	plugin.RegisterExporter("jsonl-plugin", func() probe.Exporter {
		return &JSONLExporter{W: io.Discard}
	})
}

// JSONLExporter writes one JSON object per line (example plugin-style exporter).
type JSONLExporter struct {
	W io.Writer
}

func (e *JSONLExporter) Export(report probe.ProbeReport) error {
	enc := json.NewEncoder(e.W)
	for _, step := range report.Steps {
		if err := enc.Encode(step); err != nil {
			return err
		}
	}
	return nil
}
