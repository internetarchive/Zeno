// config.go
package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/MatusOllah/slogcolor"
	"github.com/fatih/color"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log/ringbuffer"
	slogmulti "github.com/samber/slog-multi"
)

var (
	rotatedLogFile *rotatedFile
)

type logConfig struct {
	FileConfig    *logfileConfig
	StdoutEnabled bool
	StdoutLevel   slog.Level
	StderrEnabled bool
	StderrLevel   slog.Level
	NoColor       bool
	LogTUI        bool
	TUILogLevel   slog.Level
}

type logfileConfig struct {
	Dir          string
	Prefix       string
	Level        slog.Level
	Rotate       bool
	RotatePeriod time.Duration
}

// makeConfig returns the default configuration
func makeConfig() *logConfig {
	if config.Get() == nil {
		return &logConfig{
			FileConfig:    nil,
			StdoutEnabled: true,
			StdoutLevel:   slog.LevelInfo,
			StderrEnabled: true,
			StderrLevel:   slog.LevelError,
			LogTUI:        false,
		}
	}

	fileRotatePeriod, err := time.ParseDuration(config.Get().LogFileRotation)
	if err != nil && config.Get().LogFileRotation != "" {
		fileRotatePeriod = 1 * time.Hour
	}

	var logFileOutputDir string
	if logFileOutputDir = config.Get().LogFileOutputDir; logFileOutputDir == "" {
		logFileOutputDir = fmt.Sprintf("%s/logs", config.Get().JobPath)
	}

	var logFileConfig *logfileConfig
	if !config.Get().NoFileLogging {
		logFileConfig = &logfileConfig{
			Dir:          logFileOutputDir,
			Prefix:       config.Get().LogFilePrefix,
			Level:        parseLevel(config.Get().LogFileLevel),
			Rotate:       config.Get().LogFileRotation != "",
			RotatePeriod: fileRotatePeriod,
		}
	} else {
		logFileConfig = nil
	}

	return &logConfig{
		FileConfig:    logFileConfig,
		StdoutEnabled: !config.Get().NoStdoutLogging,
		StdoutLevel:   parseLevel(config.Get().StdoutLogLevel),
		StderrEnabled: !config.Get().NoStderrLogging,
		StderrLevel:   slog.LevelError,
		NoColor:       config.Get().NoColorLogging,
		LogTUI:        config.Get().TUI,
		TUILogLevel:   parseLevel(config.Get().TUILogLevel),
	}
}

func parseLevel(level string) slog.Level {
	lowercaseLevel := strings.ToLower(level)
	switch lowercaseLevel {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func newColorOptions(Level slog.Level) *slogcolor.Options {
	return &slogcolor.Options{
		Level:         Level,
		TimeFormat:    time.RFC3339,
		SrcFileMode:   slogcolor.ShortFile,
		SrcFileLength: 20,
		MsgPrefix:     color.HiWhiteString("| "),
		MsgColor:      color.New().Add(color.FgYellow),
		LevelTags:     slogcolor.DefaultLevelTags,
	}
}

func (c *logConfig) newHandler(out io.Writer, Level slog.Level) slog.Handler {
	if c.NoColor {
		return slog.NewTextHandler(out, &slog.HandlerOptions{Level: Level})
	} else {
		return slogcolor.NewHandler(out, newColorOptions(Level))
	}
}

func (c *logConfig) makeMultiLogger() *slog.Logger {
	baseRouter := slogmulti.Router()

	// Handle stdout/stderr logging configuration
	// If Stdout and Stderr are both enabled we log every level below stderr level to stdout and the rest (above) to stderr
	if c.StdoutEnabled && c.StderrEnabled {
		stderrHandler := c.newHandler(os.Stderr, c.StderrLevel)
		baseRouter = baseRouter.Add(stderrHandler, func(_ context.Context, r slog.Record) bool {
			return r.Level >= c.StderrLevel
		})

		stdoutHandler := c.newHandler(os.Stdout, c.StdoutLevel)
		baseRouter = baseRouter.Add(stdoutHandler, func(_ context.Context, r slog.Record) bool {
			return r.Level >= c.StdoutLevel && r.Level < c.StderrLevel
		})
	} else if c.StdoutEnabled {
		stdoutHandler := c.newHandler(os.Stdout, c.StdoutLevel)
		baseRouter = baseRouter.Add(stdoutHandler, func(_ context.Context, r slog.Record) bool {
			return r.Level >= c.StdoutLevel
		})
	}

	// Handle file logging configuration
	if c.FileConfig != nil {
		rotatedLogFile = newRotatedFile(c.FileConfig)
		fileHandler := slog.NewTextHandler(rotatedLogFile, &slog.HandlerOptions{Level: c.FileConfig.Level})
		baseRouter = baseRouter.Add(fileHandler, func(_ context.Context, r slog.Record) bool {
			return r.Level >= c.FileConfig.Level
		})
	}

	// Handle TUI logging configuration
	if c.LogTUI {
		TUIRingBuffer = ringbuffer.NewMP1COverwritingRingBuffer[string](16384)
		rbWriter := ringbuffer.NewWriter(TUIRingBuffer)
		rbHandler := slog.NewTextHandler(rbWriter, &slog.HandlerOptions{Level: c.TUILogLevel})
		baseRouter = baseRouter.Add(rbHandler, func(_ context.Context, r slog.Record) bool {
			return r.Level >= c.TUILogLevel
		})
	}

	// Handle Elasticsearch logging configuration
	// TODO

	return slog.New(baseRouter.Handler())
}
