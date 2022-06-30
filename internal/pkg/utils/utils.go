package utils

import (
	"fmt"
	"io"
	"os"
	"path"
	"runtime/debug"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
)

// SetupLogging setup the logger for the crawl
func SetupLogging(jobPath string, liveStats bool) (logInfo, logWarning *logrus.Logger) {
	var logsDirectory = path.Join(jobPath, "logs")
	logInfo = logrus.New()
	logWarning = logrus.New()

	// Create logs directory for the job
	os.MkdirAll(logsDirectory, os.ModePerm)

	// Initialize rotating loggers
	pathInfo := path.Join(logsDirectory, "zeno_info")
	pathWarning := path.Join(logsDirectory, "zeno_warning")

	writerInfo, err := rotatelogs.New(
		fmt.Sprintf("%s_%s.log", pathInfo, "%Y%m%d%H%M%S"),
		rotatelogs.WithRotationTime(time.Hour*6),
	)
	if err != nil {
		logrus.WithFields(logrus.Fields{"error": err}).Fatalln("Failed to initialize info log file")
	}

	if !liveStats {
		logInfo.SetOutput(io.MultiWriter(writerInfo, os.Stdout))
	} else {
		logInfo.SetOutput(writerInfo)
	}

	writerWarning, err := rotatelogs.New(
		fmt.Sprintf("%s_%s.log", pathWarning, "%Y%m%d%H%M%S"),
		rotatelogs.WithRotationTime(time.Hour*6),
	)
	if err != nil {
		logrus.WithFields(logrus.Fields{"error": err}).Fatalln("Failed to initialize warning log file")
	}

	if !liveStats {
		logWarning.SetOutput(io.MultiWriter(writerWarning, os.Stdout))
	} else {
		logWarning.SetOutput(writerWarning)
	}

	return logInfo, logWarning
}

// Defaults to "master" in case there is an error, else it returns the git hash with "-true" if the git tree on build was different
func GetVersion() string {
	version := "master"
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			// This returns the current git hash
			if setting.Key == "vcs.revision" {
				version = setting.Value
			}

			// This would show us if the current git tree is modified from the hash, possible changes that weren't committed.
			if setting.Key == "vcs.modified" {
				if setting.Value != "false" {
					version = version + "-" + setting.Value
				}
			}
		}
	}
	return version
}
