package testutil

import (
	"log/slog"
	"os"
)

// TestLogger returns a *slog.Logger configured at DEBUG level writing to stderr.
// Use it in tests to see all log output regardless of the global slog level.
func TestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}
