package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SavedProbe is a named reusable probe template (v3).
type SavedProbe struct {
	Name            string             `json:"name"`
	Description     string             `json:"description,omitempty"`
	URL             string             `json:"url"`
	Method          string             `json:"method"`
	Headers         map[string]string  `json:"headers,omitempty"`
	Body            string             `json:"body,omitempty"`
	FollowRedirects bool               `json:"follow_redirects"`
	TimeoutMS       int                `json:"timeout_ms,omitempty"`
	IgnoreSSL       bool               `json:"ignore_ssl"`
	Proxy           *string            `json:"proxy,omitempty"`
	SLO             map[string]float64 `json:"slo,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

// SavedProbeRepository manages named templates.
type SavedProbeRepository interface {
	Upsert(ctx context.Context, sp SavedProbe) (SavedProbe, error)
	Get(ctx context.Context, name string) (SavedProbe, error)
	List(ctx context.Context) ([]SavedProbe, error)
	Delete(ctx context.Context, name string) error
}

func (r *SQLiteRepository) migrateSaved() error {
	_, err := r.db.Exec(`
CREATE TABLE IF NOT EXISTS saved_probes (
  name TEXT PRIMARY KEY,
  description TEXT NOT NULL DEFAULT '',
  template_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
`)
	return err
}

func (r *SQLiteRepository) UpsertSaved(_ context.Context, sp SavedProbe) (SavedProbe, error) {
	name := strings.TrimSpace(sp.Name)
	if name == "" {
		return SavedProbe{}, fmt.Errorf("saved probe name is required")
	}
	if strings.TrimSpace(sp.URL) == "" {
		return SavedProbe{}, fmt.Errorf("saved probe url is required")
	}
	if sp.Method == "" {
		sp.Method = "GET"
	}
	now := time.Now().UTC()
	existing, err := r.GetSaved(context.Background(), name)
	created := now
	if err == nil {
		created = existing.CreatedAt
	} else if err != ErrNotFound {
		return SavedProbe{}, err
	}
	sp.Name = name
	sp.CreatedAt = created
	sp.UpdatedAt = now
	raw, err := json.Marshal(sp)
	if err != nil {
		return SavedProbe{}, err
	}
	_, err = r.db.Exec(
		`INSERT INTO saved_probes (name, description, template_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET
		   description = excluded.description,
		   template_json = excluded.template_json,
		   updated_at = excluded.updated_at`,
		name,
		sp.Description,
		string(raw),
		created.Format(time.RFC3339Nano),
		now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return SavedProbe{}, err
	}
	return sp, nil
}

func (r *SQLiteRepository) GetSaved(_ context.Context, name string) (SavedProbe, error) {
	var raw string
	err := r.db.QueryRow(`SELECT template_json FROM saved_probes WHERE name = ?`, name).Scan(&raw)
	if err == sql.ErrNoRows {
		return SavedProbe{}, ErrNotFound
	}
	if err != nil {
		return SavedProbe{}, err
	}
	var sp SavedProbe
	if err := json.Unmarshal([]byte(raw), &sp); err != nil {
		return SavedProbe{}, err
	}
	return sp, nil
}

func (r *SQLiteRepository) ListSaved(_ context.Context) ([]SavedProbe, error) {
	rows, err := r.db.Query(`SELECT template_json FROM saved_probes ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SavedProbe{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var sp SavedProbe
		if err := json.Unmarshal([]byte(raw), &sp); err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) DeleteSaved(_ context.Context, name string) error {
	res, err := r.db.Exec(`DELETE FROM saved_probes WHERE name = ?`, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
