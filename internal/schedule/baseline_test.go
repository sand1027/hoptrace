package schedule

import "testing"

func TestComputeBaseline(t *testing.T) {
	samples := []float64{200, 180, 190, 210, 500, 185, 195} // newest first includes 200
	b := ComputeBaseline("https://x", samples)
	if b.Count != 7 {
		t.Fatalf("count=%d", b.Count)
	}
	if b.P50MS <= 0 || b.P95MS < b.P50MS {
		t.Fatalf("p50=%v p95=%v", b.P50MS, b.P95MS)
	}
}
