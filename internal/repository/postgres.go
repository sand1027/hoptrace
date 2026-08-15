package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/sandeepv/hoptrace/internal/probe"
)

// PostgresRepository is a ProbeRepository adapter for Postgres (v9).
type PostgresRepository struct {
	db *sql.DB
}

// OpenPostgres opens a Postgres-backed probe repository from DATABASE_URL / dsn.
func OpenPostgres(dsn string) (*PostgresRepository, error) {
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		return nil, fmt.Errorf("postgres dsn required (DATABASE_URL)")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	r := &PostgresRepository{db: db}
	if err := r.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return r, nil
}

func (r *PostgresRepository) Close() error { return r.db.Close() }

func (r *PostgresRepository) migrate() error {
	_, err := r.db.Exec(`
CREATE TABLE IF NOT EXISTS probes (
  id TEXT PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL,
  initial_url TEXT NOT NULL,
  final_status INTEGER NOT NULL,
  total_time_ms DOUBLE PRECISION NOT NULL,
  report_json JSONB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_probes_created_at ON probes(created_at DESC);
`)
	return err
}

func (r *PostgresRepository) Save(_ context.Context, report probe.ProbeReport) (ProbeRecord, error) {
	id := newID()
	created := time.Now().UTC()
	raw, err := json.Marshal(report)
	if err != nil {
		return ProbeRecord{}, err
	}
	_, err = r.db.Exec(
		`INSERT INTO probes (id, created_at, initial_url, final_status, total_time_ms, report_json)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		id, created, report.InitialURL, report.Summary.FinalStatus, report.Summary.TotalTimeMS, raw,
	)
	if err != nil {
		return ProbeRecord{}, err
	}
	return ProbeRecord{ID: id, CreatedAt: created, Report: report}, nil
}

func (r *PostgresRepository) Get(_ context.Context, id string) (ProbeRecord, error) {
	var created time.Time
	var raw []byte
	err := r.db.QueryRow(`SELECT created_at, report_json FROM probes WHERE id=$1`, id).Scan(&created, &raw)
	if err == sql.ErrNoRows {
		return ProbeRecord{}, ErrNotFound
	}
	if err != nil {
		return ProbeRecord{}, err
	}
	var report probe.ProbeReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return ProbeRecord{}, err
	}
	return ProbeRecord{ID: id, CreatedAt: created, Report: report}, nil
}

func (r *PostgresRepository) List(_ context.Context, limit int) ([]ProbeRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(
		`SELECT id, created_at, report_json FROM probes ORDER BY created_at DESC LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProbeRecord{}
	for rows.Next() {
		var id string
		var created time.Time
		var raw []byte
		if err := rows.Scan(&id, &created, &raw); err != nil {
			return nil, err
		}
		var report probe.ProbeReport
		if err := json.Unmarshal(raw, &report); err != nil {
			return nil, err
		}
		out = append(out, ProbeRecord{ID: id, CreatedAt: created, Report: report})
	}
	return out, rows.Err()
}

var _ ProbeRepository = (*PostgresRepository)(nil)
