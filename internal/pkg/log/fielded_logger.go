package log

import (
	"context"
	"log/slog"
	"maps"
	"runtime"
	"slices"
	"time"
)

// Field defines an interface for fields
type Fields map[string]any

// FieldedLogger allows adding predefined fields to log entries
type FieldedLogger struct {
	ctx	   context.Context
	fields *[]any
}

// NewFieldedLogger creates a new FieldedLogger with the given fields
func NewFieldedLogger(args *Fields) *FieldedLogger {
	sortedArgs := make([]any, 0, len(*args)*2)
	for _, k := range slices.Sorted(maps.Keys(*args)) {
		sortedArgs = append(sortedArgs, k, (*args)[k])
	}
	return &FieldedLogger{
		ctx:    context.Background(),
		fields: &sortedArgs,
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
	combinedArgs := make([]any, 0, len(*fl.fields)+len(args))

	combinedArgs = append(combinedArgs, *fl.fields...)
	combinedArgs = append(combinedArgs, args...)

	if multiLogger != nil {
		// Code copy from [slog.Logger:log()]
		//
		// This is needed to feed the correct caller frame PC to the Record
		// since we warpped the [slog.Logger] with our own [FieldedLogger].
		// https://github.com/golang/go/issues/73707#issuecomment-2878940561
		if !multiLogger.Enabled(fl.ctx, level) {
			return
		}
		var pc uintptr
		var pcs [1]uintptr
		// skip [runtime.Callers, this function, this function's caller]
		runtime.Callers(3, pcs[:])
		pc = pcs[0]

		record := slog.NewRecord(time.Now(), level, msg, pc)
		record.Add(combinedArgs...)
		multiLogger.Handler().Handle(fl.ctx, record)
	}
}
