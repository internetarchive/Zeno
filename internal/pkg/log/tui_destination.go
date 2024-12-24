package log

import "log/slog"

// TUIDestination logs to the TUI
type TUIDestination struct {
	level slog.Level
}

// NewTUIDestination creates a new TUI destination
func NewTUIDestination() *TUIDestination {
	return &TUIDestination{
		level: slog.LevelInfo,
	}
}

func (d *TUIDestination) Enabled() bool {
	return true
}

func (d *TUIDestination) Level() slog.Level {
	return d.level
}

func (d *TUIDestination) Write(entry *logEntry) {
	LogChanTUI <- formatLogEntry(entry)
}

func (d *TUIDestination) Close() {
	// Do nothing
}
