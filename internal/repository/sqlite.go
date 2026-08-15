package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/sandeepv/hoptrace/internal/probe"
)

// SQLiteRepository persists probes on disk (Adapter for ProbeRepository).
type SQLiteRepository struct {
	db *sql.DB
}

// DefaultDBPath returns ~/.hoptrace/history.db (or HOPTRACE_DB if set).
func DefaultDBPath() (string, error) {
	if v := os.Getenv("HOPTRACE_DB"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".hoptrace")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "history.db"), nil
}

// OpenSQLite opens (or creates) a SQLite-backed repository.
func OpenSQLite(path string) (*SQLiteRepository, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	r := &SQLiteRepository{db: db}
	if err := r.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return r, nil
}

func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}

func (r *SQLiteRepository) migrate() error {
	_, err := r.db.Exec(`
CREATE TABLE IF NOT EXISTS probes (
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,
  initial_url TEXT NOT NULL,
  final_status INTEGER NOT NULL,
  total_time_ms REAL NOT NULL,
  report_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_probes_created_at ON probes(created_at DESC);
`)
	if err != nil {
		return err
	}
	if err := r.migrateSaved(); err != nil {
		return err
	}
	return r.migrateV5Plus()
}

func (r *SQLiteRepository) Save(_ context.Context, report probe.ProbeReport) (ProbeRecord, error) {
	id := newID()
	created := time.Now().UTC()
	raw, err := json.Marshal(report)
	if err != nil {
		return ProbeRecord{}, err
	}
	_, err = r.db.Exec(
		`INSERT INTO probes (id, created_at, initial_url, final_status, total_time_ms, report_json)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id,
		created.Format(time.RFC3339Nano),
		report.InitialURL,
		report.Summary.FinalStatus,
		report.Summary.TotalTimeMS,
		string(raw),
	)
	if err != nil {
		return ProbeRecord{}, err
	}
	return ProbeRecord{ID: id, CreatedAt: created, Report: report}, nil
}

func (r *SQLiteRepository) Get(_ context.Context, id string) (ProbeRecord, error) {
	var created string
	var raw string
	err := r.db.QueryRow(
		`SELECT created_at, report_json FROM probes WHERE id = ?`, id,
	).Scan(&created, &raw)
	if err == sql.ErrNoRows {
		return ProbeRecord{}, ErrNotFound
	}
	if err != nil {
		return ProbeRecord{}, err
	}
	return decodeRecord(id, created, raw)
}

func (r *SQLiteRepository) List(_ context.Context, limit int) ([]ProbeRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(
		`SELECT id, created_at, report_json FROM probes ORDER BY created_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ProbeRecord{}
	for rows.Next() {
		var id, created, raw string
		if err := rows.Scan(&id, &created, &raw); err != nil {
			return nil, err
		}
		rec, err := decodeRecord(id, created, raw)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func decodeRecord(id, created, raw string) (ProbeRecord, error) {
	var report probe.ProbeReport
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		return ProbeRecord{}, fmt.Errorf("decode report %s: %w", id, err)
	}
	ts, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		ts, err = time.Parse(time.RFC3339, created)
		if err != nil {
			ts = time.Time{}
		}
	}
	return ProbeRecord{ID: id, CreatedAt: ts, Report: report}, nil
}

func newID() string {
	return time.Now().UTC().Format("20060102T150405.000") + "-" + randSuffix()
}

func randSuffix() string {
	// lightweight unique-ish suffix without importing crypto for local IDs
	n := time.Now().UnixNano() % 1000000
	return fmt.Sprintf("%06d", n)
}

// Ensure interface compliance.
var _ ProbeRepository = (*SQLiteRepository)(nil)
