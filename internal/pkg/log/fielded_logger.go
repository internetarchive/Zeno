package log

import (
	"context"
	"log/slog"
	"runtime"
	"time"
)

// Field defines an interface for fields
type Fields map[string]interface{}

// FieldedLogger allows adding predefined fields to log entries
type FieldedLogger struct {
	fields *Fields
}

// NewFieldedLogger creates a new FieldedLogger with the given fields
func NewFieldedLogger(args *Fields) *FieldedLogger {
	return &FieldedLogger{
		fields: args,
	}
}

// Debug logs a message at the debug level with the predefined fields
func (fl *FieldedLogger) Debug(msg string, args ...any) {
	fl.logWithLevel(slog.LevelDebug, msg, args...)
}

// Info logs a message at the info level with the predefined fields
func (fl *FieldedLogger) Info(msg string, args ...any) {
	fl.logWithLevel(slog.LevelInfo, msg, args...)
}

// Warn logs a message at the warn level with the predefined fields
func (fl *FieldedLogger) Warn(msg string, args ...any) {
	fl.logWithLevel(slog.LevelWarn, msg, args...)
}

// Error logs a message at the error level with the predefined fields
func (fl *FieldedLogger) Error(msg string, args ...any) {
	fl.logWithLevel(slog.LevelError, msg, args...)
}

func (fl *FieldedLogger) logWithLevel(level slog.Level, msg string, args ...any) {
	var combinedArgs []any

	if fl.fields != nil {
		for k, v := range *fl.fields {
			combinedArgs = append(combinedArgs, k, v)
		}
	}

	combinedArgs = append(combinedArgs, args...)

	if multiLogger != nil {
		// Code copy from [slog.Logger:log()]
		//
		// This is needed to feed the correct caller frame PC to the Record
		// since we warpped the [slog.Logger] with our own [FieldedLogger].
		// https://github.com/golang/go/issues/73707#issuecomment-2878940561
		ctx := context.Background()
		if !multiLogger.Enabled(ctx, level) {
			return
		}
		var pc uintptr
		var pcs [1]uintptr
		// skip [runtime.Callers, this function, this function's caller]
		runtime.Callers(3, pcs[:])
		pc = pcs[0]

		record := slog.NewRecord(time.Now(), level, msg, pc)
		record.Add(args...)
		multiLogger.Handler().Handle(ctx, record)
	}
}
