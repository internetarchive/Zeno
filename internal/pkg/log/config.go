// config.go
package log

import (
	"context"
	"fmt"
	"io"
	stdliblog "log"
	"log/slog"
	"net"
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
	socketCfg      *socketConfig
)

type logConfig struct {
	FileConfig    *logfileConfig
	StdoutEnabled bool
	StdoutLevel   slog.Level
	StderrEnabled bool
	StderrLevel   slog.Level
	SocketEnabled bool
	SocketConfig  *socketConfig
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

type socketConfig struct {
	SocketPath  string
	Level       slog.Level
	NetListener net.Listener
	Conn        net.Conn
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

	if config.Get().SocketLogging != "" {
		socketPath := config.Get().SocketLogging
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			stdliblog.Printf("Warning: Failed to remove old socket at %s: %v", socketPath, err)
		} // Clean up any old socket
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			stdliblog.Fatalf("Failed to listen on Unix socket at %s: %v", socketPath, err)
		}
		stdliblog.Printf("Listening on Unix socket: %s", socketPath)

		// We must wait for a client to connect before we can write logs to the socket.
		// The --log-socket is only used for tests, so we can block here and only use the first connection.
		conn, err := listener.Accept()
		if err != nil {
			stdliblog.Fatalf("Failed to accept connection on Unix socket: %v", err)
		}
		stdliblog.Println("Client connected!")
		socketCfg = &socketConfig{
			Level:       parseLevel(config.Get().SocketLevel),
			SocketPath:  socketPath,
			NetListener: listener,
			Conn:        conn,
		}
	}

	return &logConfig{
		FileConfig:    logFileConfig,
		StdoutEnabled: !config.Get().NoStdoutLogging,
		StdoutLevel:   parseLevel(config.Get().StdoutLogLevel),
		StderrEnabled: !config.Get().NoStderrLogging,
		StderrLevel:   slog.LevelError,
		SocketEnabled: config.Get().SocketLogging != "",
		SocketConfig:  socketCfg,
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

	// Handle socket logging configuration
	if c.SocketEnabled && c.SocketConfig != nil {
		socketHandler := slog.NewTextHandler(c.SocketConfig.Conn, &slog.HandlerOptions{Level: c.SocketConfig.Level})
		baseRouter = baseRouter.Add(socketHandler, func(_ context.Context, r slog.Record) bool {
			return r.Level >= c.SocketConfig.Level
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
