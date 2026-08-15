package platform

import (
	"log/slog"
	"os"
	"sync"
)

var (
	once   sync.Once
	logger *slog.Logger
	level  = new(slog.LevelVar)
)

func init() {
	level.Set(slog.LevelInfo)
}

// Logger returns the process-wide structured logger (Singleton — config/logging only).
func Logger() *slog.Logger {
	once.Do(func() {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		}))
	})
	return logger
}

// SetLogLevel adjusts process log verbosity (CLI defaults quieter).
func SetLogLevel(l slog.Level) {
	level.Set(l)
	_ = Logger()
}
