// Package log provides a custom logging solution with multi-output support
package log

import (
	"context"
	"log/slog"
	"os"
	"sync"
)

var (
	defaultLogger *Logger
	once          sync.Once
)

// Logger wraps slog.Logger to provide multi-output functionality
type Logger struct {
	handler *multiHandler
	slogger *slog.Logger
}

// multiHandler implements slog.Handler interface for multiple outputs
type multiHandler struct {
	handlers []slog.Handler
}

// Config holds the configuration for the logger
type Config struct {
	FileOutput  string
	FileLevel   slog.Level
	StdoutLevel slog.Level
}

// New creates a new Logger instance with the given configuration.
// It sets up handlers for stdout (text format) and file output (JSON format) if specified.
// If FileOutput is empty, only stdout logging will be enabled.
//
// Parameters:
//   - cfg: Config struct containing logger configuration options
//
// Returns:
//   - *Logger: A new Logger instance
//   - error: An error if there was a problem creating the logger (e.g., unable to open log file)
func New(cfg Config) (*Logger, error) {
	var handlers []slog.Handler

	// Create stdout handler
	stdoutHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.StdoutLevel,
	})
	handlers = append(handlers, stdoutHandler)

	// Create file handler if FileOutput is specified
	if cfg.FileOutput != "" {
		file, err := os.OpenFile(cfg.FileOutput, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, err
		}
		jsonHandler := slog.NewJSONHandler(file, &slog.HandlerOptions{
			Level: cfg.FileLevel,
		})
		handlers = append(handlers, jsonHandler)
	}

	// Create multi-handler
	mh := &multiHandler{handlers: handlers}

	// Create slog.Logger
	slogger := slog.New(mh)

	return &Logger{handler: mh, slogger: slogger}, nil
}

// Default returns the default Logger instance.
// The default logger writes to both stdout (text format) and a file named "app.log" (JSON format).
// Both outputs are set to log messages at Info level and above.
// This function uses sync.Once to ensure that the default logger is only created once.
//
// Returns:
//   - *Logger: The default Logger instance
func Default() *Logger {
	once.Do(func() {
		logger, err := New(Config{
			FileOutput:  "zeno.log",
			FileLevel:   slog.LevelInfo,
			StdoutLevel: slog.LevelInfo,
		})
		if err != nil {
			panic(err)
		}
		defaultLogger = logger
	})
	return defaultLogger
}

// Debug logs a message at Debug level.
// The first argument is the message to log, and subsequent arguments are key-value pairs
// that will be included in the log entry.
//
// Parameters:
//   - msg: The message to log
//   - args: Optional key-value pairs to include in the log entry
func (l *Logger) Debug(msg string, args ...any) {
	l.slogger.Debug(msg, args...)
}

// Info logs a message at Info level.
// The first argument is the message to log, and subsequent arguments are key-value pairs
// that will be included in the log entry.
//
// Parameters:
//   - msg: The message to log
//   - args: Optional key-value pairs to include in the log entry
func (l *Logger) Info(msg string, args ...any) {
	l.slogger.Info(msg, args...)
}

// Warn logs a message at Warn level.
// The first argument is the message to log, and subsequent arguments are key-value pairs
// that will be included in the log entry.
//
// Parameters:
//   - msg: The message to log
//   - args: Optional key-value pairs to include in the log entry
func (l *Logger) Warn(msg string, args ...any) {
	l.slogger.Warn(msg, args...)
}

// Error logs a message at Error level.
// The first argument is the message to log, and subsequent arguments are key-value pairs
// that will be included in the log entry.
//
// Parameters:
//   - msg: The message to log
//   - args: Optional key-value pairs to include in the log entry
func (l *Logger) Error(msg string, args ...any) {
	l.slogger.Error(msg, args...)
}

// Fatal logs a message at Fatal level and then calls os.Exit(1).
// The first argument is the message to log, and subsequent arguments are key-value pairs
// that will be included in the log entry.
//
// Parameters:
//   - msg: The message to log
//   - args: Optional key-value pairs to include in the log entry
func (l *Logger) Fatal(msg string, args ...any) {
	l.slogger.Log(context.Background(), slog.LevelError, msg, args...)
	os.Exit(1)
}

//-------------------------------------------------------------------------------------
// Following methods are used to implement the slog.Handler interface for multiHandler
//-------------------------------------------------------------------------------------

// This method checks if any of the underlying handlers are enabled for a given log level.
// It's used internally to determine if a log message should be processed by a given handler
func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// This method is responsible for passing the log record to all underlying handlers.
// It's called internally when a log message needs to be written.
func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h.handlers {
		if err := handler.Handle(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

// This method creates a new handler with additional attributes.
// It's used internally when the logger is asked to include additional context with all subsequent log messages.
func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

// This method creates a new handler with a new group added to the attribute grouping hierarchy.
// It's used internally when the logger is asked to group a set of attributes together.
func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}
