package repository

import (
	"context"
	"sync"
	"time"

	"github.com/sandeepv/hoptrace/internal/probe"
)

// ProbeRecord is a stored probe run (v2+; interface ready in v1).
type ProbeRecord struct {
	ID        string            `json:"id"`
	CreatedAt time.Time         `json:"created_at"`
	Report    probe.ProbeReport `json:"report"`
}

// ProbeRepository abstracts persistence (Repository pattern).
type ProbeRepository interface {
	Save(ctx context.Context, report probe.ProbeReport) (ProbeRecord, error)
	Get(ctx context.Context, id string) (ProbeRecord, error)
	List(ctx context.Context, limit int) ([]ProbeRecord, error)
}

// MemoryRepository is an in-memory Adapter (tests / ephemeral mode).
type MemoryRepository struct {
	mu      sync.RWMutex
	records []ProbeRecord
	seq     int
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{records: []ProbeRecord{}}
}

func (r *MemoryRepository) Save(_ context.Context, report probe.ProbeReport) (ProbeRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	rec := ProbeRecord{
		ID:        formatID(r.seq),
		CreatedAt: time.Now().UTC(),
		Report:    report,
	}
	r.records = append([]ProbeRecord{rec}, r.records...)
	return rec, nil
}

func (r *MemoryRepository) Get(_ context.Context, id string) (ProbeRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, rec := range r.records {
		if rec.ID == id {
			return rec, nil
		}
	}
	return ProbeRecord{}, ErrNotFound
}

func (r *MemoryRepository) List(_ context.Context, limit int) ([]ProbeRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 || limit > len(r.records) {
		limit = len(r.records)
	}
	out := make([]ProbeRecord, limit)
	copy(out, r.records[:limit])
	return out, nil
}

// NoopRepository discards all writes (Null Object).
type NoopRepository struct{}

func (NoopRepository) Save(_ context.Context, report probe.ProbeReport) (ProbeRecord, error) {
	return ProbeRecord{Report: report}, nil
}
func (NoopRepository) Get(context.Context, string) (ProbeRecord, error) { return ProbeRecord{}, ErrNotFound }
func (NoopRepository) List(context.Context, int) ([]ProbeRecord, error)  { return nil, nil }

// ErrNotFound is returned when a probe id does not exist.
var ErrNotFound = errNotFound{}

type errNotFound struct{}

func (errNotFound) Error() string { return "probe not found" }

func formatID(n int) string {
	return time.Now().UTC().Format("20060102T150405") + "-" + itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
