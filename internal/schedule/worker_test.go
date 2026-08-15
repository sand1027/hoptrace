package schedule

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sandeepv/hoptrace/internal/probe"
)

type memStore struct {
	jobs []Job
}

func (m *memStore) ListJobs(context.Context) ([]Job, error) { return m.jobs, nil }
func (m *memStore) GetJob(context.Context, string) (Job, error) {
	return Job{}, nil
}
func (m *memStore) UpsertJob(_ context.Context, job Job) (Job, error) { return job, nil }
func (m *memStore) DeleteJob(context.Context, string) error           { return nil }
func (m *memStore) TouchJob(context.Context, string, int, float64, string) error {
	return nil
}

func TestWorker_RespectsMaxInFlightAndLease(t *testing.T) {
	var running int32
	var peak int32
	store := &memStore{jobs: []Job{
		{ID: "a", Name: "a", URL: "https://example.com", IntervalSec: 1, Enabled: true},
		{ID: "b", Name: "b", URL: "https://example.com", IntervalSec: 1, Enabled: true},
		{ID: "c", Name: "c", URL: "https://example.com", IntervalSec: 1, Enabled: true},
		{ID: "d", Name: "d", URL: "https://example.com", IntervalSec: 1, Enabled: true},
		{ID: "e", Name: "e", URL: "https://example.com", IntervalSec: 1, Enabled: true},
	}}
	w := NewWorker(store, func(ctx context.Context, job Job) (probe.ProbeReport, error) {
		n := atomic.AddInt32(&running, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		time.Sleep(80 * time.Millisecond)
		atomic.AddInt32(&running, -1)
		return probe.ProbeReport{Summary: probe.Summary{FinalStatus: 200}}, nil
	}, nil)
	w.MaxInFlight = 2
	w.TickEvery = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)
	time.Sleep(250 * time.Millisecond)
	w.Stop()
	if peak > 2 {
		t.Fatalf("peak concurrency %d > max 2", peak)
	}
}
