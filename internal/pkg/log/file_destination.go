package log

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"sync"
	"time"
)

// FileDestination logs to a file with rotation
type FileDestination struct {
	level     slog.Level
	config    *LogfileConfig
	file      *os.File
	mu        sync.Mutex
	ticker    *time.Ticker
	closeChan chan struct{}
}

func NewFileDestination() *FileDestination {
	fd := &FileDestination{
		level:     globalConfig.FileConfig.Level,
		config:    globalConfig.FileConfig,
		closeChan: make(chan struct{}),
	}

	fd.rotateFile()
	if globalConfig.FileConfig.Rotate && globalConfig.FileConfig.RotatePeriod > 0 {
		fd.ticker = time.NewTicker(globalConfig.FileConfig.RotatePeriod)
		go fd.rotationWorker()
	}

	return fd
}

func (d *FileDestination) Enabled() bool {
	return true
}

func (d *FileDestination) Level() slog.Level {
	return d.level
}

func (d *FileDestination) Write(entry *logEntry) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.file != nil {
		fmt.Fprintln(d.file, formatLogEntry(entry))
	}
}

func (d *FileDestination) Close() {
	if d.ticker != nil {
		d.ticker.Stop()
	}
	close(d.closeChan)
	d.mu.Lock()
	if d.file != nil {
		fmt.Fprintln(d.file, "Log file closed")
		d.file.Close()
	}
	d.mu.Unlock()
}

func (d *FileDestination) rotateFile() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.file != nil {
		d.file.Close()
	}

	// Check if the directory exists, if not create it
	if _, err := os.Stat(d.config.Dir); os.IsNotExist(err) {
		err = os.MkdirAll(d.config.Dir, 0755)
		if err != nil {
			log.Fatalf("Failed to create log directory: %v", err)
		}
	}

	filename := fmt.Sprintf("%s/%s-%s.log", d.config.Dir, d.config.Prefix, time.Now().Format("2006.01.02T15-04"))
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	d.file = file
}

func (d *FileDestination) rotationWorker() {
	for {
		select {
		case <-d.ticker.C:
			d.rotateFile()
		case <-d.closeChan:
			return
		}
	}
}
