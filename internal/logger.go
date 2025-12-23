package internal

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// SetupLogger configures structured logging with JSON format
// Logs are written to both console (text) and file (JSON)
func SetupLogger(logDir string) (*slog.Logger, error) {
	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	// Create log file
	logFile := filepath.Join(logDir, "relay.log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	// Create multi-writer: console (text) + file (JSON)
	consoleHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	fileHandler := slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	// Combine handlers using a custom multi-handler
	multiHandler := &MultiHandler{
		handlers: []slog.Handler{consoleHandler, fileHandler},
	}

	logger := slog.New(multiHandler)
	slog.SetDefault(logger)

	return logger, nil
}

// MultiHandler sends log records to multiple handlers
type MultiHandler struct {
	handlers []slog.Handler
}

func (h *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *MultiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, handler := range h.handlers {
		if err := handler.Handle(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func (h *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &MultiHandler{handlers: handlers}
}

func (h *MultiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &MultiHandler{handlers: handlers}
}

// LogWriter wraps slog.Logger to implement io.Writer for compatibility
type LogWriter struct {
	logger *slog.Logger
	level  slog.Level
}

func NewLogWriter(logger *slog.Logger, level slog.Level) *LogWriter {
	return &LogWriter{
		logger: logger,
		level:  level,
	}
}

func (w *LogWriter) Write(p []byte) (n int, err error) {
	w.logger.Log(context.Background(), w.level, string(p))
	return len(p), nil
}

var _ io.Writer = (*LogWriter)(nil)
