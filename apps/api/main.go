package main

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/websocket"

	"github.com/sandeepv/hoptrace/internal/auth"
	"github.com/sandeepv/hoptrace/internal/notify"
	"github.com/sandeepv/hoptrace/internal/platform"
	"github.com/sandeepv/hoptrace/internal/probe"
	"github.com/sandeepv/hoptrace/internal/ratelimit"
	"github.com/sandeepv/hoptrace/internal/repository"
	"github.com/sandeepv/hoptrace/internal/schedule"
	"github.com/sandeepv/hoptrace/internal/setup"
	"github.com/sandeepv/hoptrace/internal/ssrf"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	allowPrivate := flag.Bool("allow-private", false, "allow probes to private/loopback IPs")
	dbPath := flag.String("db", "", "sqlite db path")
	requireAuth := flag.Bool("require-auth", false, "require API keys even when none exist")
	rps := flag.Float64("rps", 10, "per-key requests per second")
	flag.Parse()

	logger := platform.Logger()
	path := *dbPath
	if path == "" {
		var err error
		path, err = repository.DefaultDBPath()
		if err != nil {
			logger.Error("db path", "err", err)
			os.Exit(1)
		}
	}
	repo, err := repository.OpenSQLite(path)
	if err != nil {
		logger.Error("open sqlite", "err", err)
		os.Exit(1)
	}
	defer repo.Close()
	logger.Info("history db", "path", path)

	api := &server{
		repo:   repo,
		guard:  ssrf.NewGuard(*allowPrivate),
		logger: logger,
		limiter: ratelimit.New(*rps, int(*rps*2)),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	worker := schedule.NewWorker(repo, api.runJob, logger)
	worker.OnComplete = api.onScheduleComplete
	worker.Start(ctx)
	defer worker.Stop()

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		MaxAge:         300,
	}))

	r.Get("/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"store":   "sqlite",
			"db":      path,
			"version": "v9",
		})
	})
	r.Get("/v1/share/{token}", api.getShare) // public

	r.Route("/v1", func(r chi.Router) {
		r.Use(auth.Middleware(repo, *requireAuth))
		r.Use(ratelimit.Middleware(api.limiter, func(req *http.Request) string {
			return auth.WorkspaceID(req.Context())
		}))

		r.With(chimw.Timeout(60*time.Second)).Post("/probes", api.createProbe)
		r.With(chimw.Timeout(15*time.Second)).Get("/probes", api.listProbes)
		r.Get("/probes/stream", api.streamProbe)
		r.With(chimw.Timeout(15*time.Second)).Get("/probes/{id}", api.getProbe)
		r.With(chimw.Timeout(15*time.Second)).Post("/probes/{id}/share", api.createShare)

		r.With(chimw.Timeout(15 * time.Second)).Route("/saved", func(r chi.Router) {
			r.Get("/", api.listSaved)
			r.Post("/", api.upsertSaved)
			r.Get("/{name}", api.getSaved)
			r.Put("/{name}", api.upsertSavedNamed)
			r.Delete("/{name}", api.deleteSaved)
			r.Post("/{name}/run", api.runSaved)
		})

		r.With(chimw.Timeout(15 * time.Second)).Route("/schedules", func(r chi.Router) {
			r.Get("/", api.listSchedules)
			r.Post("/", api.upsertSchedule)
			r.Get("/{id}", api.getSchedule)
			r.Delete("/{id}", api.deleteSchedule)
		})

		r.With(chimw.Timeout(15*time.Second)).Get("/baselines", api.baselines)
		r.With(chimw.Timeout(15*time.Second)).Get("/failures", api.listFailures)

		r.With(chimw.Timeout(15 * time.Second)).Route("/keys", func(r chi.Router) {
			r.Get("/", api.listKeys)
			r.Post("/", api.createKey)
			r.Delete("/{id}", api.deleteKey)
		})
	})

	srv := &http.Server{Addr: *addr, Handler: r}
	go func() {
		logger.Info("api listening", "addr", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancel()
	shCtx, shCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shCancel()
	_ = srv.Shutdown(shCtx)
}

type server struct {
	repo    *repository.SQLiteRepository
	guard   *ssrf.Guard
	logger  platformLogger
	limiter *ratelimit.Limiter
}

// thin alias so we don't fight slog types in struct
type platformLogger = interface {
	Info(string, ...any)
	Error(string, ...any)
	Warn(string, ...any)
}

type probeAPIRequest struct {
	URL             string             `json:"url"`
	Method          string             `json:"method"`
	Headers         map[string]string  `json:"headers"`
	Body            string             `json:"body"`
	FollowRedirects bool               `json:"follow_redirects"`
	TimeoutMS       int                `json:"timeout_ms"`
	IgnoreSSL       bool               `json:"ignore_ssl"`
	Proxy           *string            `json:"proxy"`
	SLO             map[string]float64 `json:"slo"`
	WebhookURL      string             `json:"webhook_url"`
	SlackWebhook    string             `json:"slack_webhook"`
}

type probeAPIResponse struct {
	ID     string            `json:"id,omitempty"`
	Report probe.ProbeReport `json:"report"`
	Error  *string           `json:"error"`
}

func (s *server) createProbe(w http.ResponseWriter, r *http.Request) {
	var body probeAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	req, err := buildRequest(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.guard.ValidateURL(req.URL); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	report, probeErr := setup.DefaultAnalyzer().Analyze(r.Context(), req)
	rec, _ := s.repo.Save(r.Context(), report)
	s.maybeAlert(r.Context(), "probe", body, report, probeErr)
	status := http.StatusOK
	if probeErr != nil {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, probeAPIResponse{ID: rec.ID, Report: report, Error: errString(probeErr)})
}

func (s *server) listProbes(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	list, err := s.repo.List(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) getProbe(w http.ResponseWriter, r *http.Request) {
	rec, err := s.repo.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		o := r.Header.Get("Origin")
		return o == "" || o == "http://localhost:3000" || o == "http://127.0.0.1:3000"
	},
}

func (s *server) streamProbe(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	var body probeAPIRequest
	if err := conn.ReadJSON(&body); err != nil {
		_ = conn.WriteJSON(probe.Event{Kind: "error", Error: "invalid request json"})
		return
	}
	req, err := buildRequest(body)
	if err != nil {
		_ = conn.WriteJSON(probe.Event{Kind: "error", Error: err.Error()})
		return
	}
	if err := s.guard.ValidateURL(req.URL); err != nil {
		_ = conn.WriteJSON(probe.Event{Kind: "error", Error: err.Error()})
		return
	}
	listener := probe.NewChanListener(64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ev := range listener.Ch {
			if err := conn.WriteJSON(ev); err != nil {
				return
			}
		}
	}()
	report, probeErr := setup.DefaultAnalyzer(probe.WithEvents(listener)).Analyze(r.Context(), req)
	rec, _ := s.repo.Save(r.Context(), report)
	listener.Close()
	<-done
	s.maybeAlert(r.Context(), "probe-stream", body, report, probeErr)
	_ = conn.WriteJSON(map[string]any{"kind": "complete", "id": rec.ID, "report": report, "error": errString(probeErr)})
}

func (s *server) listSaved(w http.ResponseWriter, r *http.Request) {
	list, err := s.repo.ListSaved(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) getSaved(w http.ResponseWriter, r *http.Request) {
	sp, err := s.repo.GetSaved(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, sp)
}

func (s *server) upsertSaved(w http.ResponseWriter, r *http.Request) {
	var sp repository.SavedProbe
	if err := json.NewDecoder(r.Body).Decode(&sp); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	out, err := s.repo.UpsertSaved(r.Context(), sp)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) upsertSavedNamed(w http.ResponseWriter, r *http.Request) {
	var sp repository.SavedProbe
	if err := json.NewDecoder(r.Body).Decode(&sp); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	sp.Name = chi.URLParam(r, "name")
	out, err := s.repo.UpsertSaved(r.Context(), sp)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) deleteSaved(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.DeleteSaved(r.Context(), chi.URLParam(r, "name")); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *server) runSaved(w http.ResponseWriter, r *http.Request) {
	sp, err := s.repo.GetSaved(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	body := probeAPIRequest{
		URL: sp.URL, Method: sp.Method, Headers: sp.Headers, Body: sp.Body,
		FollowRedirects: sp.FollowRedirects, TimeoutMS: sp.TimeoutMS, IgnoreSSL: sp.IgnoreSSL,
		Proxy: sp.Proxy, SLO: sp.SLO,
	}
	req, err := buildRequest(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.guard.ValidateURL(req.URL); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	report, probeErr := setup.DefaultAnalyzer().Analyze(r.Context(), req)
	rec, _ := s.repo.Save(r.Context(), report)
	s.maybeAlert(r.Context(), "saved:"+sp.Name, body, report, probeErr)
	status := http.StatusOK
	if probeErr != nil {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, probeAPIResponse{ID: rec.ID, Report: report, Error: errString(probeErr)})
}

func (s *server) listSchedules(w http.ResponseWriter, r *http.Request) {
	list, err := s.repo.ListJobs(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) getSchedule(w http.ResponseWriter, r *http.Request) {
	job, err := s.repo.GetJob(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *server) upsertSchedule(w http.ResponseWriter, r *http.Request) {
	var job schedule.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	job.WorkspaceID = auth.WorkspaceID(r.Context())
	out, err := s.repo.UpsertJob(r.Context(), job)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.DeleteJob(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *server) baselines(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url query required"})
		return
	}
	samples, err := s.repo.TotalsForURL(r.Context(), url, 50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, schedule.ComputeBaseline(url, samples))
}

func (s *server) listFailures(w http.ResponseWriter, r *http.Request) {
	ws := auth.WorkspaceID(r.Context())
	list, err := s.repo.ListFailures(r.Context(), ws, 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) listKeys(w http.ResponseWriter, r *http.Request) {
	list, err := s.repo.ListKeys(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) createKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		WorkspaceID string `json:"workspace_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if body.Name == "" {
		body.Name = "default"
	}
	if body.WorkspaceID == "" {
		body.WorkspaceID = auth.WorkspaceID(r.Context())
	}
	plain, prefix, hash, err := auth.GenerateKey()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	rec, err := s.repo.Create(r.Context(), body.Name, body.WorkspaceID, prefix, hash)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"key":        plain, // shown once
		"id":         rec.ID,
		"prefix":     rec.Prefix,
		"workspace":  rec.WorkspaceID,
		"created_at": rec.CreatedAt,
	})
}

func (s *server) deleteKey(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.DeleteKey(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *server) createShare(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := s.repo.Get(r.Context(), id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	token, err := s.repo.CreateShare(r.Context(), id, auth.WorkspaceID(r.Context()))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"token": token,
		"path":  "/v1/share/" + token,
	})
}

func (s *server) getShare(w http.ResponseWriter, r *http.Request) {
	probeID, err := s.repo.ResolveShare(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	rec, err := s.repo.Get(r.Context(), probeID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *server) runJob(ctx context.Context, job schedule.Job) (probe.ProbeReport, error) {
	body := probeAPIRequest{
		URL: job.URL, Method: job.Method, Headers: job.Headers, Body: job.Body,
		FollowRedirects: job.FollowRedirects, TimeoutMS: job.TimeoutMS, SLO: job.SLO,
	}
	if job.SavedProbeName != "" {
		sp, err := s.repo.GetSaved(ctx, job.SavedProbeName)
		if err != nil {
			return probe.ProbeReport{}, err
		}
		body.URL = sp.URL
		body.Method = sp.Method
		body.Headers = sp.Headers
		body.Body = sp.Body
		body.FollowRedirects = sp.FollowRedirects
		body.TimeoutMS = sp.TimeoutMS
		body.SLO = sp.SLO
		body.IgnoreSSL = sp.IgnoreSSL
		body.Proxy = sp.Proxy
	}
	req, err := buildRequest(body)
	if err != nil {
		return probe.ProbeReport{}, err
	}
	if err := s.guard.ValidateURL(req.URL); err != nil {
		return probe.ProbeReport{}, err
	}
	report, runErr := setup.DefaultAnalyzer().Analyze(ctx, req)
	_, _ = s.repo.Save(ctx, report)
	return report, runErr
}

func (s *server) onScheduleComplete(ctx context.Context, job schedule.Job, report probe.ProbeReport, err error) {
	body := probeAPIRequest{WebhookURL: job.WebhookURL, SlackWebhook: job.SlackWebhook, URL: report.InitialURL}
	s.maybeAlert(ctx, "schedule:"+job.Name, body, report, err)
}

func (s *server) maybeAlert(ctx context.Context, title string, body probeAPIRequest, report probe.ProbeReport, runErr error) {
	ev, ok := notify.FromReport(title, report, runErr)
	if !ok {
		return
	}
	ws := auth.WorkspaceID(ctx)
	_ = s.repo.RecordFailure(ctx, ws, ev.Title, ev.Message, ev.URL, ev.Status, ev.TotalMS)
	n := notify.MultiNotifier{Items: []notify.Notifier{
		notify.WebhookNotifier{URL: body.WebhookURL},
		notify.SlackNotifier{WebhookURL: body.SlackWebhook},
	}}
	if err := n.Notify(ctx, ev); err != nil {
		s.logger.Warn("notify failed", "err", err)
	}
}

func buildRequest(body probeAPIRequest) (probe.ProbeRequest, error) {
	b := probe.NewRequestBuilder().URL(body.URL)
	if body.Method != "" {
		b.Method(body.Method)
	}
	if body.Headers != nil {
		b.Headers(body.Headers)
	}
	if body.Body != "" {
		b.Body([]byte(body.Body))
	}
	b.FollowRedirect(body.FollowRedirects)
	if body.TimeoutMS > 0 {
		b.Timeout(time.Duration(body.TimeoutMS) * time.Millisecond)
	}
	if body.IgnoreSSL {
		b.IgnoreSSL(true)
	}
	if body.Proxy != nil {
		b.Proxy(*body.Proxy)
	}
	if body.SLO != nil {
		b.SLO(body.SLO)
	}
	return b.Build()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errString(err error) *string {
	if err == nil {
		return nil
	}
	s := err.Error()
	return &s
}

// ensure platform logger satisfies our interface at compile time via usage in main
var _ = platform.Logger
