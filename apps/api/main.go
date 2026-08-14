package main

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/sandeepv/hoptrace/internal/platform"
	"github.com/sandeepv/hoptrace/internal/probe"
	"github.com/sandeepv/hoptrace/internal/repository"
	"github.com/sandeepv/hoptrace/internal/setup"
	"github.com/sandeepv/hoptrace/internal/ssrf"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	allowPrivate := flag.Bool("allow-private", false, "allow probes to private/loopback IPs (dev only)")
	flag.Parse()

	logger := platform.Logger()
	repo := repository.NewMemoryRepository()
	guard := ssrf.NewGuard(*allowPrivate)
	analyzer := setup.DefaultAnalyzer()

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Post("/v1/probes", func(w http.ResponseWriter, r *http.Request) {
		var body probeAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}

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

		req, err := b.Build()
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		if err := guard.ValidateURL(req.URL); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}

		report, probeErr := analyzer.Analyze(r.Context(), req)
		rec, _ := repo.Save(r.Context(), report)

		status := http.StatusOK
		if probeErr != nil {
			status = http.StatusBadGateway
		}
		writeJSON(w, status, probeAPIResponse{
			ID:     rec.ID,
			Report: report,
			Error:  errString(probeErr),
		})
	})

	r.Get("/v1/probes", func(w http.ResponseWriter, r *http.Request) {
		list, err := repo.List(r.Context(), 50)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, list)
	})

	r.Get("/v1/probes/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		rec, err := repo.Get(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, rec)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

type probeAPIRequest struct {
	URL             string            `json:"url"`
	Method          string            `json:"method"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	FollowRedirects bool              `json:"follow_redirects"`
	TimeoutMS       int               `json:"timeout_ms"`
	IgnoreSSL       bool              `json:"ignore_ssl"`
	Proxy           *string           `json:"proxy"`
	SLO             map[string]float64 `json:"slo"`
}

type probeAPIResponse struct {
	ID     string            `json:"id,omitempty"`
	Report probe.ProbeReport `json:"report"`
	Error  *string           `json:"error"`
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
