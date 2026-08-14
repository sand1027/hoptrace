package platform

import (
	"log/slog"
	"os"
	"sync"
)

var (
	once   sync.Once
	logger *slog.Logger
)

// Logger returns the process-wide structured logger (Singleton — config/logging only).
func Logger() *slog.Logger {
	once.Do(func() {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	})
	return logger
}
