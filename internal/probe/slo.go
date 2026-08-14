package probe

import "strings"

// SLOEvaluator checks phase timings against thresholds (Strategy).
type SLOEvaluator struct{}

func NewSLOEvaluator() *SLOEvaluator { return &SLOEvaluator{} }

// Evaluate runs SLO checks against the final successful step.
// If thresholds is empty, returns nil (no SLO block).
func (e *SLOEvaluator) Evaluate(thresholds map[string]float64, step StepResult) *SLOResult {
	if len(thresholds) == 0 {
		return nil
	}
	result := &SLOResult{
		Pass:         true,
		ThresholdsMS: copyFloatMap(thresholds),
		Violations:   []SLOViolation{},
	}
	for key, limit := range thresholds {
		actual := phaseValue(key, step.Timing)
		if actual > limit {
			result.Pass = false
			result.Violations = append(result.Violations, SLOViolation{
				Key:         key,
				ThresholdMS: limit,
				ActualMS:    actual,
				DeltaMS:     actual - limit,
			})
		}
	}
	return result
}

func phaseValue(key string, t Timing) float64 {
	switch strings.ToLower(key) {
	case "dns":
		return t.DNSMS
	case "connect":
		return t.ConnectMS
	case "tls":
		return t.TLSMS
	case "ttfb":
		return t.TTFBMS
	case "wait":
		return t.WaitMS
	case "xfer":
		return t.XferMS
	case "total":
		return t.TotalMS
	default:
		return 0
	}
}

func copyFloatMap(m map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
