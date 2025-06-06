package log

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

type rotatedFile struct {
	config    *logfileConfig
	file      *os.File
	mu        sync.Mutex
	ticker    *time.Ticker
	closeChan chan struct{}
}

func newRotatedFile(config *logfileConfig) *rotatedFile {
	rfile := &rotatedFile{
		config:    config,
		closeChan: make(chan struct{}),
	}

	rfile.rotateFile()
	if rfile.config.Rotate && rfile.config.RotatePeriod > 0 {
		rfile.ticker = time.NewTicker(rfile.config.RotatePeriod)
		wg.Add(1)
		go rfile.rotationWorker()
	}

	return rfile
}

func (d *rotatedFile) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.file == nil {
		return 0, os.ErrClosed
	}
	return d.file.Write(p)
}

func (d *rotatedFile) Close() {
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

func (d *rotatedFile) rotateFile() {
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

func (d *rotatedFile) rotationWorker() {
	defer wg.Done()
	for {
		select {
		case <-d.ticker.C:
			d.rotateFile()
		case <-d.closeChan:
			return
		}
	}
}
