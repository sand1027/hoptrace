package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sandeepv/hoptrace/internal/auth"
	"github.com/sandeepv/hoptrace/internal/schedule"
)

func (r *SQLiteRepository) migrateV5Plus() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS schedules (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL DEFAULT 'default',
  job_json TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  updated_at TEXT NOT NULL
);`,
		`CREATE TABLE IF NOT EXISTS api_keys (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  prefix TEXT NOT NULL,
  workspace_id TEXT NOT NULL,
  key_hash TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL
);`,
		`CREATE TABLE IF NOT EXISTS share_links (
  token TEXT PRIMARY KEY,
  probe_id TEXT NOT NULL,
  workspace_id TEXT NOT NULL DEFAULT 'default',
  created_at TEXT NOT NULL
);`,
		`CREATE TABLE IF NOT EXISTS failures (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL DEFAULT 'default',
  created_at TEXT NOT NULL,
  title TEXT NOT NULL,
  message TEXT NOT NULL,
  url TEXT NOT NULL,
  status INTEGER NOT NULL,
  total_ms REAL NOT NULL
);`,
	}
	for _, s := range stmts {
		if _, err := r.db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLiteRepository) ListJobs(_ context.Context) ([]schedule.Job, error) {
	rows, err := r.db.Query(`SELECT job_json FROM schedules ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []schedule.Job{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var j schedule.Job
		if err := json.Unmarshal([]byte(raw), &j); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) GetJob(_ context.Context, id string) (schedule.Job, error) {
	var raw string
	err := r.db.QueryRow(`SELECT job_json FROM schedules WHERE id = ?`, id).Scan(&raw)
	if err == sql.ErrNoRows {
		return schedule.Job{}, ErrNotFound
	}
	if err != nil {
		return schedule.Job{}, err
	}
	var j schedule.Job
	if err := json.Unmarshal([]byte(raw), &j); err != nil {
		return schedule.Job{}, err
	}
	return j, nil
}

func (r *SQLiteRepository) UpsertJob(_ context.Context, job schedule.Job) (schedule.Job, error) {
	if strings.TrimSpace(job.Name) == "" {
		return schedule.Job{}, fmt.Errorf("schedule name required")
	}
	if job.IntervalSec <= 0 {
		return schedule.Job{}, fmt.Errorf("interval_sec must be > 0")
	}
	if job.URL == "" && job.SavedProbeName == "" {
		return schedule.Job{}, fmt.Errorf("url or saved_probe_name required")
	}
	now := time.Now().UTC()
	if job.ID == "" {
		job.ID = "sch-" + time.Now().UTC().Format("20060102T150405") + "-" + randSuffix()
		job.CreatedAt = now
	}
	if job.Method == "" {
		job.Method = "GET"
	}
	if job.WorkspaceID == "" {
		job.WorkspaceID = "default"
	}
	job.UpdatedAt = now
	raw, err := json.Marshal(job)
	if err != nil {
		return schedule.Job{}, err
	}
	enabled := 0
	if job.Enabled {
		enabled = 1
	}
	_, err = r.db.Exec(
		`INSERT INTO schedules (id, workspace_id, job_json, enabled, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET job_json=excluded.job_json, workspace_id=excluded.workspace_id,
		   enabled=excluded.enabled, updated_at=excluded.updated_at`,
		job.ID, job.WorkspaceID, string(raw), enabled, now.Format(time.RFC3339Nano),
	)
	return job, err
}

func (r *SQLiteRepository) DeleteJob(_ context.Context, id string) error {
	res, err := r.db.Exec(`DELETE FROM schedules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLiteRepository) TouchJob(ctx context.Context, id string, status int, totalMS float64, errMsg string) error {
	job, err := r.GetJob(ctx, id)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	job.LastRunAt = &now
	job.LastStatus = status
	job.LastTotalMS = totalMS
	job.LastError = errMsg
	job.UpdatedAt = now
	_, err = r.UpsertJob(ctx, job)
	return err
}

func (r *SQLiteRepository) FindByHash(_ context.Context, hash string) (auth.KeyRecord, error) {
	var rec auth.KeyRecord
	err := r.db.QueryRow(
		`SELECT id, name, prefix, workspace_id, key_hash, created_at FROM api_keys WHERE key_hash = ?`, hash,
	).Scan(&rec.ID, &rec.Name, &rec.Prefix, &rec.WorkspaceID, &rec.KeyHash, &rec.CreatedAt)
	if err == sql.ErrNoRows {
		return auth.KeyRecord{}, ErrNotFound
	}
	return rec, err
}

func (r *SQLiteRepository) Create(ctx context.Context, name, workspaceID, prefix, hash string) (auth.KeyRecord, error) {
	id := "key-" + randSuffix()
	created := time.Now().UTC().Format(time.RFC3339Nano)
	if workspaceID == "" {
		workspaceID = "default"
	}
	_, err := r.db.Exec(
		`INSERT INTO api_keys (id, name, prefix, workspace_id, key_hash, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, name, prefix, workspaceID, hash, created,
	)
	if err != nil {
		return auth.KeyRecord{}, err
	}
	return auth.KeyRecord{ID: id, Name: name, Prefix: prefix, WorkspaceID: workspaceID, CreatedAt: created}, nil
}

func (r *SQLiteRepository) ListKeys(_ context.Context) ([]auth.KeyRecord, error) {
	rows, err := r.db.Query(`SELECT id, name, prefix, workspace_id, created_at FROM api_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []auth.KeyRecord{}
	for rows.Next() {
		var rec auth.KeyRecord
		if err := rows.Scan(&rec.ID, &rec.Name, &rec.Prefix, &rec.WorkspaceID, &rec.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) DeleteKey(_ context.Context, id string) error {
	res, err := r.db.Exec(`DELETE FROM api_keys WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLiteRepository) Count(_ context.Context) (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM api_keys`).Scan(&n)
	return n, err
}

func (r *SQLiteRepository) CreateShare(_ context.Context, probeID, workspaceID string) (string, error) {
	token := "shr-" + randSuffix() + randSuffix()
	if workspaceID == "" {
		workspaceID = "default"
	}
	_, err := r.db.Exec(
		`INSERT INTO share_links (token, probe_id, workspace_id, created_at) VALUES (?, ?, ?, ?)`,
		token, probeID, workspaceID, time.Now().UTC().Format(time.RFC3339Nano),
	)
	return token, err
}

func (r *SQLiteRepository) ResolveShare(_ context.Context, token string) (string, error) {
	var probeID string
	err := r.db.QueryRow(`SELECT probe_id FROM share_links WHERE token = ?`, token).Scan(&probeID)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return probeID, err
}

func (r *SQLiteRepository) RecordFailure(_ context.Context, workspaceID, title, message, url string, status int, totalMS float64) error {
	if workspaceID == "" {
		workspaceID = "default"
	}
	id := "fail-" + randSuffix()
	_, err := r.db.Exec(
		`INSERT INTO failures (id, workspace_id, created_at, title, message, url, status, total_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, workspaceID, time.Now().UTC().Format(time.RFC3339Nano), title, message, url, status, totalMS,
	)
	return err
}

type FailureRecord struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	CreatedAt   string  `json:"created_at"`
	Title       string  `json:"title"`
	Message     string  `json:"message"`
	URL         string  `json:"url"`
	Status      int     `json:"status"`
	TotalMS     float64 `json:"total_ms"`
}

func (r *SQLiteRepository) ListFailures(_ context.Context, workspaceID string, limit int) ([]FailureRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.Query(
		`SELECT id, workspace_id, created_at, title, message, url, status, total_ms
		 FROM failures WHERE workspace_id = ? OR ? = ''
		 ORDER BY created_at DESC LIMIT ?`,
		workspaceID, workspaceID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FailureRecord{}
	for rows.Next() {
		var f FailureRecord
		if err := rows.Scan(&f.ID, &f.WorkspaceID, &f.CreatedAt, &f.Title, &f.Message, &f.URL, &f.Status, &f.TotalMS); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// TotalsForURL returns newest-first total_ms samples for baseline.
func (r *SQLiteRepository) TotalsForURL(_ context.Context, url string, limit int) ([]float64, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(
		`SELECT total_time_ms FROM probes WHERE initial_url = ? ORDER BY created_at DESC LIMIT ?`,
		url, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []float64{}
	for rows.Next() {
		var v float64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

var (
	_ schedule.Store = (*SQLiteRepository)(nil)
	_ auth.KeyStore  = (*SQLiteRepository)(nil)
)
