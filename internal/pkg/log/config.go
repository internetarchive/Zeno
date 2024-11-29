// config.go
package log

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/config"
)

// Config defines the configuration for the logging package
type Config struct {
	FileConfig          *LogfileConfig
	StdoutEnabled       bool
	StdoutLevel         slog.Level
	StderrEnabled       bool
	StderrLevel         slog.Level
	ElasticsearchConfig *ElasticsearchConfig
}

// LogfileConfig defines the configuration for file logging
type LogfileConfig struct {
	Dir          string
	Prefix       string
	Level        slog.Level
	Rotate       bool
	RotatePeriod time.Duration
}

// ElasticsearchConfig defines the configuration for Elasticsearch logging
type ElasticsearchConfig struct {
	Addresses    string
	Username     string
	Password     string
	IndexPrefix  string
	Level        slog.Level
	Rotate       bool
	RotatePeriod time.Duration
}

// makeConfig returns the default configuration
func makeConfig() *Config {
	if config.Get() == nil {
		return &Config{
			FileConfig:          nil,
			StdoutEnabled:       true,
			StdoutLevel:         slog.LevelInfo,
			StderrEnabled:       true,
			StderrLevel:         slog.LevelError,
			ElasticsearchConfig: nil,
		}
	}
	fileRotatePeriod, err := time.ParseDuration(config.Get().LogFileRotation)
	if err != nil && config.Get().LogFileRotation != "" {
		fileRotatePeriod = 1 * time.Hour
	}

	elasticRotatePeriod, err := time.ParseDuration(config.Get().ElasticSearchRotation)
	if err != nil && config.Get().ElasticSearchRotation != "" {
		elasticRotatePeriod = 24 * time.Hour
	}

	var logFileOutputDir string
	if logFileOutputDir = config.Get().LogFileOutputDir; logFileOutputDir == "" {
		logFileOutputDir = fmt.Sprintf("%s/logs", config.Get().JobPath)
	}

	return &Config{
		FileConfig: &LogfileConfig{
			Dir:          logFileOutputDir,
			Prefix:       config.Get().LogFilePrefix,
			Level:        parseLevel(config.Get().LogFileLevel),
			Rotate:       config.Get().LogFileRotation != "",
			RotatePeriod: fileRotatePeriod,
		},
		ElasticsearchConfig: &ElasticsearchConfig{
			Addresses:    config.Get().ElasticSearchURLs,
			Username:     config.Get().ElasticSearchUsername,
			Password:     config.Get().ElasticSearchPassword,
			IndexPrefix:  config.Get().ElasticSearchIndexPrefix,
			Level:        parseLevel(config.Get().ElasticSearchLogLevel),
			Rotate:       config.Get().ElasticSearchRotation != "",
			RotatePeriod: elasticRotatePeriod,
		},
		StdoutEnabled: !config.Get().NoStdoutLogging,
		StdoutLevel:   parseLevel(config.Get().StdoutLogLevel),
		StderrEnabled: !config.Get().NoStderrLogging,
		StderrLevel:   parseLevel("error"),
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
