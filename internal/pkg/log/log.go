// log.go
package log

import (
	"log/slog"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/log/ringbuffer"
)

// Global variables
var (
	once        sync.Once
	wg          sync.WaitGroup
	multiLogger *slog.Logger

	TUIRingBuffer *ringbuffer.MP1COverwritingRingBuffer[string]
)

// Start initializes the logging package with the given configuration.
// If no configuration is provided, it uses the default configuration.
func Start() error {
	var done = false

	once.Do(func() {
		config := makeConfig()
		multiLogger = config.makeMultiLogger()
		done = true
	})

	if !done {
		return ErrLoggerAlreadyInitialized
	}

	return nil
}

// Stop gracefully shuts down the logging system
func Stop() {
	if rotatedLogFile != nil {
		rotatedLogFile.Close()
	}
	wg.Wait()
	multiLogger = nil
	once = sync.Once{}
}

// Debug logs a message at the debug level
func Debug(msg string, args ...any) {
	if multiLogger != nil {
		multiLogger.Debug(msg, args...)
	}
}

// Info logs a message at the info level
func Info(msg string, args ...any) {
	if multiLogger != nil {
		multiLogger.Info(msg, args...)
	}
}

// Warn logs a message at the warn level
func Warn(msg string, args ...any) {
	if multiLogger != nil {
		multiLogger.Warn(msg, args...)
	}
}

// Error logs a message at the error level
func Error(msg string, args ...any) {
	if multiLogger != nil {
		multiLogger.Error(msg, args...)
	}
}
