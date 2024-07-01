package log

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

type fileHandler struct {
	slog.Handler
	filename         string
	file             *os.File
	rotationInterval time.Duration
	lastRotation     time.Time
	level            slog.Level
	logfile          *Logfile
}

type Logfile struct {
	Dir    string
	Prefix string
}

func (h *fileHandler) Rotate() error {
	if h.file != nil {
		h.file.Close()
	}

	h.filename = h.logfile.Filename()

	file, err := os.OpenFile(h.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open new log file: %w", err)
	}

	h.file = file
	h.Handler = slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: h.level,
	})

	h.lastRotation = time.Now()
	return nil
}

func (h *fileHandler) NextRotation() time.Time {
	return h.lastRotation.Add(h.rotationInterval)
}

func (s *Logfile) Filename() string {
	return fmt.Sprintf("%s/%s.%s.log", s.Dir, s.Prefix, time.Now().Format("2006.01.02-15h"))
}
