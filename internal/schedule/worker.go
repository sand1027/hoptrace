package schedule

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/sandeepv/hoptrace/internal/probe"
)

const (
	DefaultMaxInFlight = 4
	MinIntervalSec     = 5
)

// Job is a recurring probe definition.
type Job struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	SavedProbeName  string             `json:"saved_probe_name,omitempty"`
	URL             string             `json:"url"`
	Method          string             `json:"method"`
	Headers         map[string]string  `json:"headers,omitempty"`
	Body            string             `json:"body,omitempty"`
	FollowRedirects bool               `json:"follow_redirects"`
	TimeoutMS       int                `json:"timeout_ms,omitempty"`
	SLO             map[string]float64 `json:"slo,omitempty"`
	IntervalSec     int                `json:"interval_sec"`
	Enabled         bool               `json:"enabled"`
	WorkspaceID     string             `json:"workspace_id,omitempty"`
	WebhookURL      string             `json:"webhook_url,omitempty"`
	SlackWebhook    string             `json:"slack_webhook,omitempty"`
	LastRunAt       *time.Time         `json:"last_run_at,omitempty"`
	LastStatus      int                `json:"last_status,omitempty"`
	LastTotalMS     float64            `json:"last_total_ms,omitempty"`
	LastError       string             `json:"last_error,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

// Runner executes a single scheduled probe (injected for testability).
type Runner func(ctx context.Context, job Job) (probe.ProbeReport, error)

// OnComplete is called after each run (alerting hook).
type OnComplete func(ctx context.Context, job Job, report probe.ProbeReport, err error)

// Store persists schedule jobs.
type Store interface {
	ListJobs(ctx context.Context) ([]Job, error)
	GetJob(ctx context.Context, id string) (Job, error)
	UpsertJob(ctx context.Context, job Job) (Job, error)
	DeleteJob(ctx context.Context, id string) error
	TouchJob(ctx context.Context, id string, status int, totalMS float64, errMsg string) error
}

// Worker runs enabled jobs on an interval with bounded concurrency + per-job lease.
type Worker struct {
	Store       Store
	Run         Runner
	OnComplete  OnComplete
	Logger      *slog.Logger
	TickEvery   time.Duration
	MaxInFlight int

	mu       sync.Mutex
	lastFire map[string]time.Time
	active   map[string]struct{}
	sem      chan struct{}
	stop     chan struct{}
	wg       sync.WaitGroup
}

func NewWorker(store Store, run Runner, log *slog.Logger) *Worker {
	if log == nil {
		log = slog.Default()
	}
	max := DefaultMaxInFlight
	return &Worker{
		Store:       store,
		Run:         run,
		Logger:      log,
		TickEvery:   5 * time.Second,
		MaxInFlight: max,
		lastFire:    map[string]time.Time{},
		active:      map[string]struct{}{},
		sem:         make(chan struct{}, max),
		stop:        make(chan struct{}),
	}
}

func (w *Worker) Start(ctx context.Context) {
	if w.MaxInFlight <= 0 {
		w.MaxInFlight = DefaultMaxInFlight
	}
	if cap(w.sem) != w.MaxInFlight {
		w.sem = make(chan struct{}, w.MaxInFlight)
	}
	go w.loop(ctx)
}

func (w *Worker) Stop() {
	select {
	case <-w.stop:
	default:
		close(w.stop)
	}
	w.wg.Wait()
}

func (w *Worker) loop(ctx context.Context) {
	t := time.NewTicker(w.TickEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	jobs, err := w.Store.ListJobs(ctx)
	if err != nil {
		w.Logger.Error("schedule.list", "err", err)
		return
	}
	now := time.Now()
	for _, job := range jobs {
		if !job.Enabled {
			continue
		}
		interval := job.IntervalSec
		if interval < MinIntervalSec {
			interval = MinIntervalSec
		}
		w.mu.Lock()
		_, running := w.active[job.ID]
		last := w.lastFire[job.ID]
		w.mu.Unlock()
		if running {
			continue
		}
		if !last.IsZero() && now.Sub(last) < time.Duration(interval)*time.Second {
			continue
		}
		if job.LastRunAt != nil && now.Sub(*job.LastRunAt) < time.Duration(interval)*time.Second {
			continue
		}

		select {
		case w.sem <- struct{}{}:
		default:
			// at global concurrency cap — skip until next tick
			continue
		}

		w.mu.Lock()
		if _, running := w.active[job.ID]; running {
			w.mu.Unlock()
			<-w.sem
			continue
		}
		w.active[job.ID] = struct{}{}
		w.lastFire[job.ID] = now
		w.mu.Unlock()

		job := job
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			defer func() { <-w.sem }()
			defer func() {
				w.mu.Lock()
				delete(w.active, job.ID)
				w.mu.Unlock()
			}()
			w.execute(ctx, job)
		}()
	}
}

func (w *Worker) execute(ctx context.Context, job Job) {
	w.Logger.Info("schedule.run", "id", job.ID, "name", job.Name)
	report, err := w.Run(ctx, job)
	status := 0
	total := 0.0
	errMsg := ""
	if len(report.Steps) > 0 {
		status = report.Summary.FinalStatus
		total = report.Summary.TotalTimeMS
	}
	if err != nil {
		errMsg = err.Error()
	}
	_ = w.Store.TouchJob(ctx, job.ID, status, total, errMsg)
	if w.OnComplete != nil {
		w.OnComplete(ctx, job, report, err)
	}
}

// BaselineStats holds percentile stats for an URL.
type BaselineStats struct {
	URL       string  `json:"url"`
	Count     int     `json:"count"`
	P50MS     float64 `json:"p50_ms"`
	P95MS     float64 `json:"p95_ms"`
	MeanMS    float64 `json:"mean_ms"`
	LastMS    float64 `json:"last_ms"`
	Regressed bool    `json:"regressed"`
	Message   string  `json:"message,omitempty"`
}

// ComputeBaseline from recent total_ms samples (sorted ascending expected).
func ComputeBaseline(url string, samples []float64) BaselineStats {
	out := BaselineStats{URL: url, Count: len(samples)}
	if len(samples) == 0 {
		return out
	}
	xs := append([]float64(nil), samples...)
	for i := 0; i < len(xs); i++ {
		for j := i + 1; j < len(xs); j++ {
			if xs[j] < xs[i] {
				xs[i], xs[j] = xs[j], xs[i]
			}
		}
	}
	sum := 0.0
	for _, v := range xs {
		sum += v
	}
	out.MeanMS = sum / float64(len(xs))
	out.P50MS = percentile(xs, 50)
	out.P95MS = percentile(xs, 95)
	out.LastMS = samples[0]
	if out.LastMS > out.P95MS && len(xs) >= 5 {
		out.Regressed = true
		out.Message = "last run above p95 baseline"
	}
	return out
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := (p / 100) * float64(len(sorted)-1)
	i := int(rank)
	frac := rank - float64(i)
	if i+1 >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	return sorted[i]*(1-frac) + sorted[i+1]*frac
}
