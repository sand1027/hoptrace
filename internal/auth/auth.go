package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

type ctxKey string

const (
	CtxAPIKey      ctxKey = "api_key"
	CtxWorkspaceID ctxKey = "workspace_id"
)

// KeyRecord is a stored API key metadata (hash only).
type KeyRecord struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Prefix      string `json:"prefix"`
	WorkspaceID string `json:"workspace_id"`
	KeyHash     string `json:"-"`
	CreatedAt   string `json:"created_at"`
}

// KeyStore looks up API keys.
type KeyStore interface {
	FindByHash(ctx context.Context, hash string) (KeyRecord, error)
	Create(ctx context.Context, name, workspaceID, prefix, hash string) (KeyRecord, error)
	ListKeys(ctx context.Context) ([]KeyRecord, error)
	DeleteKey(ctx context.Context, id string) error
	Count(ctx context.Context) (int, error)
}

// GenerateKey returns plaintext key (shown once) and its hash/prefix.
func GenerateKey() (plaintext, prefix, hash string, err error) {
	b := make([]byte, 24)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", err
	}
	plaintext = "ht_" + hex.EncodeToString(b)
	prefix = plaintext[:10]
	sum := sha256.Sum256([]byte(plaintext))
	hash = hex.EncodeToString(sum[:])
	return plaintext, prefix, hash, nil
}

func HashKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// Middleware enforces API keys when requireAuth is true, or when a key is presented.
// If no key is sent and requireAuth is false, requests continue as workspace "default"
// (local/dev friendly). Presenting an invalid key always fails.
func Middleware(store KeyStore, requireAuth bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				if requireAuth {
					n, err := store.Count(r.Context())
					if err != nil {
						http.Error(w, `{"error":"auth store error"}`, http.StatusInternalServerError)
						return
					}
					if n > 0 {
						http.Error(w, `{"error":"missing api key (Authorization: Bearer ht_... or X-API-Key)"}`, http.StatusUnauthorized)
						return
					}
				}
				ctx := context.WithValue(r.Context(), CtxWorkspaceID, "default")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			rec, err := store.FindByHash(r.Context(), HashKey(token))
			if err != nil {
				http.Error(w, `{"error":"invalid api key"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), CtxAPIKey, rec.ID)
			ctx = context.WithValue(ctx, CtxWorkspaceID, rec.WorkspaceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractToken(r *http.Request) string {
	if k := r.Header.Get("X-API-Key"); k != "" {
		return strings.TrimSpace(k)
	}
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	// WebSocket clients can't always set headers — allow query param.
	if k := r.URL.Query().Get("api_key"); k != "" {
		return strings.TrimSpace(k)
	}
	return ""
}

func WorkspaceID(ctx context.Context) string {
	if v, ok := ctx.Value(CtxWorkspaceID).(string); ok && v != "" {
		return v
	}
	return "default"
}
