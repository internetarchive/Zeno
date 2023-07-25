package utils

import (
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/internetarchive/elogrus"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/olivere/elastic/v7"
	"github.com/sirupsen/logrus"
)

// SetupLogging setup the logger for the crawl
func SetupLogging(jobPath string, liveStats bool, esURL string) (logInfo, logWarning, logError *logrus.Logger) {
	var logsDirectory = path.Join(jobPath, "logs")

	hostname, err := os.Hostname()
	if err != nil {
		logrus.Panic(err)
	}

	logInfo = logrus.New()
	logWarning = logrus.New()
	logError = logrus.New()

	logInfo.SetFormatter(&logrus.JSONFormatter{})
	logWarning.SetFormatter(&logrus.JSONFormatter{})
	logError.SetFormatter(&logrus.JSONFormatter{})

	if esURL != "" {
		client, err := elastic.NewClient(elastic.SetURL(esURL))
		if err != nil {
			logrus.Panic(err)
		}

		go func() {
			hookInfo, err := elogrus.NewAsyncElasticHook(client, hostname, logrus.InfoLevel, "zeno-"+time.Now().Format("2006.01.02"))
			if err != nil {
				logrus.Panic(err)
			}

			hookWarning, err := elogrus.NewAsyncElasticHook(client, hostname, logrus.WarnLevel, "zeno-"+time.Now().Format("2006.01.02"))
			if err != nil {
				logrus.Panic(err)
			}

			hookError, err := elogrus.NewAsyncElasticHook(client, hostname, logrus.ErrorLevel, "zeno-"+time.Now().Format("2006.01.02"))
			if err != nil {
				logrus.Panic(err)
			}

			logInfo.Hooks.Add(hookInfo)
			logWarning.Hooks.Add(hookWarning)
			logError.Hooks.Add(hookError)
		}()
	}

	// Create logs directory for the job
	os.MkdirAll(logsDirectory, os.ModePerm)

	// Initialize rotating loggers
	pathInfo := path.Join(logsDirectory, "zeno_info")
	pathWarning := path.Join(logsDirectory, "zeno_warning")
	pathError := path.Join(logsDirectory, "zeno_error")

	writerInfo, err := rotatelogs.New(
		fmt.Sprintf("%s_%s.log", pathInfo, "%Y%m%d%H%M%S"),
		rotatelogs.WithRotationTime(time.Hour*6),
	)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err.Error(),
		}).Fatalln("failed to initialize info log file")
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
		logrus.WithFields(logrus.Fields{
			"err": err.Error(),
		}).Fatalln("failed to initialize warning log file")
	}

	if !liveStats {
		logWarning.SetOutput(io.MultiWriter(writerWarning, os.Stdout))
	} else {
		logWarning.SetOutput(writerWarning)
	}

	writerError, err := rotatelogs.New(
		fmt.Sprintf("%s_%s.log", pathError, "%Y%m%d%H%M%S"),
		rotatelogs.WithRotationTime(time.Hour*6),
	)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err.Error(),
		}).Fatalln("failed to initialize error log file")
	}

	if !liveStats {
		logError.SetOutput(io.MultiWriter(writerError, os.Stdout))
	} else {
		logError.SetOutput(writerError)
	}

	return logInfo, logWarning, logError
}
