// Package applog sets up the application-level structured logger. It writes
// to ~/.cftunnelkit/logs/app.log (rotating) AND stdout so dev output is
// visible in the terminal.
package applog

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Init opens the log file and sets the default slog logger. Call once at
// startup before any other logging.
func Init() (*slog.Logger, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".cftunnelkit", "logs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "app.log")

	fileWriter := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    2, // MB
		MaxBackups: 3,
		MaxAge:     0,
		Compress:   false,
	}

	multi := io.MultiWriter(os.Stdout, fileWriter)
	logger := slog.New(slog.NewTextHandler(multi, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)
	return logger, nil
}

// Dir returns the logs directory for the "open log folder" UI action.
func Dir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".cftunnelkit", "logs")
}
