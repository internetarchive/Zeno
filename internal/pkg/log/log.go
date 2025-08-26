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
	loggerMu    sync.RWMutex

	TUIRingBuffer *ringbuffer.MP1COverwritingRingBuffer[string]
)

// Start initializes the logging package with the given configuration.
// If no configuration is provided, it uses the default configuration.
func Start() error {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	once.Do(func() {
		config := makeConfig()
		multiLogger = config.makeMultiLogger()
	})
	if multiLogger == nil {
		return ErrLoggerAlreadyInitialized
	}

	return nil
}

// Stop gracefully shuts down the logging system
func Stop() {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	if rotatedLogFile != nil {
		rotatedLogFile.Close()
	}

	E2eConnMutex.RLock()
	if E2EConnCfg != nil {
		E2EConnCfg.connW.Close()
	}
	E2eConnMutex.RUnlock()

	wg.Wait()
	multiLogger = nil
	once = sync.Once{}
}

// Debug logs a message at the debug level
func Debug(msg string, args ...any) {
	loggerMu.RLock()
	defer loggerMu.RUnlock()

	if multiLogger != nil {
		multiLogger.Debug(msg, args...)
	}
}

// Info logs a message at the info level
func Info(msg string, args ...any) {
	loggerMu.RLock()
	defer loggerMu.RUnlock()

	if multiLogger != nil {
		multiLogger.Info(msg, args...)
	}
}

// Warn logs a message at the warn level
func Warn(msg string, args ...any) {
	loggerMu.RLock()
	defer loggerMu.RUnlock()

	if multiLogger != nil {
		multiLogger.Warn(msg, args...)
	}
}

// Error logs a message at the error level
func Error(msg string, args ...any) {
	loggerMu.RLock()
	defer loggerMu.RUnlock()

	if multiLogger != nil {
		multiLogger.Error(msg, args...)
	}
}
