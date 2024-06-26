// Package log provides a custom logging solution with multi-output support
package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
)

var (
	defaultLogger *Logger
	once          sync.Once
)

// Logger wraps slog.Logger to provide multi-output functionality
type Logger struct {
	handler      *multiHandler
	slogger      *slog.Logger
	mu           sync.Mutex
	stopRotation chan struct{}
	stopErrorLog chan struct{}
	errorChan    chan error
}

// Config holds the configuration for the logger
type Config struct {
	FileOutput               *Logfile
	FileLevel                slog.Level
	StdoutLevel              slog.Level
	RotateLogFile            bool
	ElasticsearchConfig      *ElasticsearchConfig
	RotateElasticSearchIndex bool
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
	if cfg.FileOutput != nil {
		// Create directories if they don't exist
		err := os.MkdirAll(filepath.Dir(cfg.FileOutput.Filename()), 0755)
		if err != nil {
			return nil, err
		}

		// Open log file
		file, err := os.OpenFile(cfg.FileOutput.Filename(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, err
		}
		fileHandler := &fileHandler{
			Handler:          slog.NewJSONHandler(file, &slog.HandlerOptions{Level: cfg.FileLevel}),
			filename:         cfg.FileOutput.Filename(),
			file:             file,
			rotationInterval: 6 * time.Hour,
			lastRotation:     time.Now(),
		}
		handlers = append(handlers, fileHandler)
	}

	// Create Elasticsearch handler if ElasticsearchConfig is specified
	if cfg.ElasticsearchConfig != nil {
		esClient, err := elasticsearch.NewClient(elasticsearch.Config{
			Addresses: cfg.ElasticsearchConfig.Addresses,
			Username:  cfg.ElasticsearchConfig.Username,
			Password:  cfg.ElasticsearchConfig.Password,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Elasticsearch client: %w", err)
		}
		esHandler := &ElasticsearchHandler{
			client: esClient,
			index:  fmt.Sprintf("%s-%s", cfg.ElasticsearchConfig.IndexPrefix, time.Now().Format("2006.01.02")),
			level:  cfg.ElasticsearchConfig.Level,
			attrs:  []slog.Attr{},
			groups: []string{},
			config: cfg.ElasticsearchConfig,
		}
		if err := esHandler.createIndex(); err != nil {
			return nil, fmt.Errorf("failed to create Elasticsearch index: %w", err)
		}
		handlers = append(handlers, esHandler)
	}

	// Create multi-handler
	mh := &multiHandler{handlers: handlers}

	// Create slog.Logger
	slogger := slog.New(mh)

	logger := &Logger{
		handler:      mh,
		slogger:      slogger,
		errorChan:    make(chan error, 10),
		stopErrorLog: make(chan struct{}),
	}

	// Start rotation goroutine
	logger.startRotation()

	return logger, nil
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
			FileOutput:  &Logfile{Dir: "jobs", Prefix: "zeno"},
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

// Errors returns a channel that will receive logging errors
func (l *Logger) Errors() <-chan error {
	return l.errorChan
}

func (l *Logger) log(level slog.Level, msg string, args ...any) {
	// Create a new Record with the message and args
	r := slog.NewRecord(time.Now(), level, msg, 0)
	r.Add(args...)

	err := l.handler.Handle(context.Background(), r)
	if err != nil {
		select {
		case l.errorChan <- err:
		default:
			// If the error channel is full, log to stderr as a last resort
			fmt.Fprintf(os.Stderr, "Logging error: %v\n", err)
		}
	}
}

// Debug logs a message at Debug level.
// The first argument is the message to log, and subsequent arguments are key-value pairs
// that will be included in the log entry.
//
// Parameters:
//   - msg: The message to log
//   - args: Optional key-value pairs to include in the log entry
func (l *Logger) Debug(msg string, args ...any) {
	l.log(slog.LevelDebug, msg, args...)
}

// Info logs a message at Info level.
// The first argument is the message to log, and subsequent arguments are key-value pairs
// that will be included in the log entry.
//
// Parameters:
//   - msg: The message to log
//   - args: Optional key-value pairs to include in the log entry
func (l *Logger) Info(msg string, args ...any) {
	l.log(slog.LevelInfo, msg, args...)
}

// Warn logs a message at Warn level.
// The first argument is the message to log, and subsequent arguments are key-value pairs
// that will be included in the log entry.
//
// Parameters:
//   - msg: The message to log
//   - args: Optional key-value pairs to include in the log entry
func (l *Logger) Warn(msg string, args ...any) {
	l.log(slog.LevelWarn, msg, args...)
}

// Error logs a message at Error level.
// The first argument is the message to log, and subsequent arguments are key-value pairs
// that will be included in the log entry.
//
// Parameters:
//   - msg: The message to log
//   - args: Optional key-value pairs to include in the log entry
func (l *Logger) Error(msg string, args ...any) {
	l.log(slog.LevelError, msg, args...)
}

// Fatal logs a message at Error level and then calls os.Exit(1).
// The first argument is the message to log, and subsequent arguments are key-value pairs
// that will be included in the log entry.
//
// Parameters:
//   - msg: The message to log
//   - args: Optional key-value pairs to include in the log entry
func (l *Logger) Fatal(msg string, args ...any) {
	l.log(slog.LevelError, msg, args...)
	os.Exit(1)
}
