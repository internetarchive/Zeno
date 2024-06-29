package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// WithFields returns a new log entry with the given fields.
// The fields are key-value pairs that will be included only in the next log entry.
//
// This method returns a log Entry, which can be used to log a message with the specified fields.
//
// Parameters:
//   - fields: A map of key-value pairs to be included in the next log entry
//
// Returns:
//   - Entry: A log entry that can be used to log a message with the specified fields
//
// Example:
//
//	logger := log.Default()
//	logger.WithFields(map[string]interface{}{
//	    "user_id": 12345,
//	    "ip": "192.168.1.1",
//	}).Info("User logged in")
func (l *Logger) WithFields(fields map[string]interface{}) *Entry {
	attrs := make([]slog.Attr, 0, len(fields))
	for k, v := range fields {
		attrs = append(attrs, slog.Any(k, v))
	}
	return &Entry{logger: l, attrs: attrs}
}

// Entry is a log entry with fields.
type Entry struct {
	logger *Logger
	attrs  []slog.Attr
}

func (e *Entry) log(ctx context.Context, level slog.Level, msg string, args ...any) {
	allAttrs := append(e.attrs, slog.Any("msg", msg))
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			allAttrs = append(allAttrs, slog.Any(fmt.Sprint(args[i]), args[i+1]))
		}
	}
	r := slog.NewRecord(time.Now(), level, msg, 0)
	r.AddAttrs(allAttrs...)
	_ = e.logger.handler.Handle(ctx, r)
}

// Info logs a message at Info level with the fields specified in WithFields.
func (e *Entry) Info(msg string, args ...any) {
	e.log(context.Background(), slog.LevelInfo, msg, args...)
}

// Warn logs a message at Warn level with the fields specified in WithFields.
func (e *Entry) Warn(msg string, args ...any) {
	e.log(context.Background(), slog.LevelWarn, msg, args...)
}

// Error logs a message at Error level with the fields specified in WithFields.
func (e *Entry) Error(msg string, args ...any) {
	e.log(context.Background(), slog.LevelError, msg, args...)
}

// Debug logs a message at Debug level with the fields specified in WithFields.
func (e *Entry) Debug(msg string, args ...any) {
	e.log(context.Background(), slog.LevelDebug, msg, args...)
}

// Fatal logs a message at Fatal level with the fields specified in WithFields, then calls os.Exit(1).
func (e *Entry) Fatal(msg string, args ...any) {
	e.log(context.Background(), slog.LevelError, msg, args...)
	os.Exit(1)
}
