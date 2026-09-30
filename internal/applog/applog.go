// Package applog sets up the application-level structured logger. It writes
// to ~/.cftunnelkit/logs/app.log (rotating) AND stdout so dev output is
// visible in the terminal. Operation logs go to operation.log.
package applog

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

var opLogMu sync.Mutex
var opLogWriter *lumberjack.Logger

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

	// operation.log writer
	opLogWriter = &lumberjack.Logger{
		Filename:   filepath.Join(dir, "operation.log"),
		MaxSize:    2,
		MaxBackups: 3,
		Compress:   false,
	}

	return logger, nil
}

// OpLog appends one line to operation.log.
func OpLog(action, result string) {
	if opLogWriter == nil {
		return
	}
	opLogMu.Lock()
	defer opLogMu.Unlock()
	line := fmt.Sprintf("%s [%s] %s\n", time.Now().Format("2006-01-02 15:04:05.000"), result, action)
	_, _ = opLogWriter.Write([]byte(line))
}

// Dir returns the logs directory for the "open log folder" UI action.
func Dir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".cftunnelkit", "logs")
}
