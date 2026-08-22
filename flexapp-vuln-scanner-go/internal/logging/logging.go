// Package logging builds the process-wide slog.Logger: every log line
// goes to both stderr (so it still shows up in a console/service
// manager) and a file next to wherever the app already keeps its
// database and TLS files, at whatever verbosity Config.LogLevel says.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"flexapp-vuln-scanner/internal/config"
)

// New opens cfg.LogFile (creating it if needed, appending if it already
// exists) and returns a logger that writes every record to both stderr
// and that file. The returned io.Closer must be closed on shutdown; the
// logger remains safe to use (falls back to stderr only) if closed
// early, since callers are expected to defer the close until process
// exit.
func New(cfg config.Config) (*slog.Logger, io.Closer, error) {
	file, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file %s: %w", cfg.LogFile, err)
	}

	level, err := parseLevel(cfg.LogLevel)
	if err != nil {
		file.Close()
		return nil, nil, err
	}

	writer := io.MultiWriter(os.Stderr, file)
	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{Level: level})
	return slog.New(handler), file, nil
}

// parseLevel maps Config.LogLevel's validated set ("debug", "info",
// "warn", "error" -- see config.Load) to an slog.Level. Anything else is
// a programming error, not a user input to recover from: Config.Load
// already rejects any other value before this is ever called.
func parseLevel(logLevel string) (slog.Level, error) {
	switch logLevel {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q", logLevel)
	}
}
