package log

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

// ... (previous Logger, multiHandler, logEntry, ElasticsearchHandler definitions remain the same)

type rotateableHandler interface {
	slog.Handler
	Rotate() error
	NextRotation() time.Time
}

type fileHandler struct {
	slog.Handler
	filename     string
	file         *os.File
	interval     time.Duration
	lastRotation time.Time
}

func (h *fileHandler) Rotate() error {
	// ... (previous Rotate implementation remains the same)
	h.lastRotation = time.Now()
	return nil
}

func (h *fileHandler) NextRotation() time.Time {
	return h.lastRotation.Add(h.interval)
}

func (h *ElasticsearchHandler) NextRotation() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(24 * time.Hour)
}

func (l *Logger) startRotation() {
	l.stopRotation = make(chan struct{})
	go func() {
		for {
			nextRotation := l.nextRotation()
			select {
			case <-time.After(time.Until(nextRotation)):
				l.rotate()
			case <-l.stopRotation:
				return
			}
		}
	}()
}

func (l *Logger) nextRotation() time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()

	var earliest time.Time
	for _, h := range l.handler.handlers {
		if rh, ok := h.(rotateableHandler); ok {
			next := rh.NextRotation()
			if earliest.IsZero() || next.Before(earliest) {
				earliest = next
			}
		}
	}
	return earliest
}

func (l *Logger) rotate() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for _, h := range l.handler.handlers {
		if rh, ok := h.(rotateableHandler); ok {
			if now.After(rh.NextRotation()) || now.Equal(rh.NextRotation()) {
				if err := rh.Rotate(); err != nil {
					fmt.Printf("Error rotating handler: %v\n", err)
				}
			}
		}
	}
}

// Stop stops the rotation goroutine
func (l *Logger) Stop() {
	close(l.stopRotation)
}

// ... (rest of the code remains the same)
