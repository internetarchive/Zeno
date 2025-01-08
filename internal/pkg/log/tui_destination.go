package log

import (
	"log/slog"

	"github.com/internetarchive/Zeno/internal/pkg/log/ringbuffer"
)

// TUIDestination logs to the TUI
type TUIDestination struct {
	buffer *ringbuffer.MP1COverwritingRingBuffer[string]
	level  slog.Level
}

// NewTUIDestination creates a new TUI destination
func NewTUIDestination() *TUIDestination {
	buffer := ringbuffer.NewMP1COverwritingRingBuffer[string](16384)
	TUIRingBuffer = buffer
	return &TUIDestination{
		buffer: buffer,
		level:  globalConfig.StdoutLevel,
	}
}

func (d *TUIDestination) Enabled() bool {
	return true
}

func (d *TUIDestination) Level() slog.Level {
	return d.level
}

func (d *TUIDestination) Write(entry *logEntry) {
	d.buffer.Enqueue(formatLogEntry(entry))
}

func (d *TUIDestination) Close() {
	// Nothing to do
	return
}
