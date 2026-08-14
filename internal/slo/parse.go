package slo

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseSpec parses "total=500,ttfb=200" into a threshold map.
// This package intentionally has no dependency on probe (avoids import cycles).
func ParseSpec(spec string) (map[string]float64, error) {
	out := map[string]float64{}
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return out, nil
	}
	parts := strings.Split(spec, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid slo fragment %q (want key=ms)", part)
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		valStr := strings.TrimSpace(kv[1])
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil || val <= 0 {
			return nil, fmt.Errorf("invalid slo value for %s: %s", key, valStr)
		}
		if _, exists := out[key]; exists {
			return nil, fmt.Errorf("duplicate slo key: %s", key)
		}
		switch key {
		case "dns", "connect", "tls", "ttfb", "wait", "xfer", "total":
			out[key] = val
		default:
			return nil, fmt.Errorf("unknown slo key: %s", key)
		}
	}
	return out, nil
}
