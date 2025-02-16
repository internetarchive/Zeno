package ringbuffer

import (
	"log/slog"
	"strings"
	"testing"
)

// TestSlogHandlerSingleLine tests that a simple log entry written via slog
// produces a complete log line in the ring buffer.
func TestSlogHandlerSingleLine(t *testing.T) {
	rb := NewMP1COverwritingRingBuffer[string](10)
	writer := NewWriter(rb)
	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{
		// For testing, you might disable timestamp, source, etc.
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			return a
		},
	})
	logger := slog.New(handler)

	// Log a simple message.
	logger.Info("test message", "key", "value")

	// In a typical use-case, the handler writes a complete log line
	// (ending with a newline). But if for some reason the log output is
	// buffered, we can call Flush to force any incomplete line.
	writer.Flush()

	// Dump from the ring buffer.
	entries := rb.DumpN(10)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(entries))
	}
	// The formatted log line should contain the message.
	if !strings.Contains(entries[0], "test message") {
		t.Errorf("expected log line to contain 'test message', got: %s", entries[0])
	}
}

// TestMultipleLinesOneWrite tests that a single Write call containing multiple
// newlines produces multiple entries.
func TestMultipleLinesOneWrite(t *testing.T) {
	rb := NewMP1COverwritingRingBuffer[string](10)
	writer := NewWriter(rb)

	// Write a string that contains three complete lines.
	input := "first line\nsecond line\nthird line\n"
	n, err := writer.Write([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(input) {
		t.Fatalf("expected to write %d bytes, wrote %d", len(input), n)
	}
	writer.Flush() // not strictly necessary here since all lines end with \n

	entries := rb.DumpN(10)
	if len(entries) != 3 {
		t.Fatalf("expected 3 log lines, got %d", len(entries))
	}

	expected := []string{"first line", "second line", "third line"}
	for i, exp := range expected {
		if entries[i] != exp {
			t.Errorf("line %d: expected %q, got %q", i, exp, entries[i])
		}
	}
}

// TestIncompleteLine tests that incomplete lines remain buffered until a newline
// is received.
func TestIncompleteLine(t *testing.T) {
	rb := NewMP1COverwritingRingBuffer[string](10)
	writer := NewWriter(rb)

	// Write an incomplete line (no newline yet).
	writer.Write([]byte("incomplete"))
	// Dumping now should return nil because no complete line is present.
	if entries := rb.DumpN(10); entries != nil {
		t.Fatalf("expected no complete log line, got: %v", entries)
	}

	// Write the rest of the line.
	writer.Write([]byte(" line\n"))
	// Now the buffered content should yield a complete line.
	entries := rb.DumpN(10)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(entries))
	}
	if entries[0] != "incomplete line" {
		t.Errorf("expected log line %q, got %q", "incomplete line", entries[0])
	}
}

// TestFlushIncomplete tests that calling Flush forces any incomplete log line
// into the ring buffer.
func TestFlushIncomplete(t *testing.T) {
	rb := NewMP1COverwritingRingBuffer[string](10)
	writer := NewWriter(rb)

	// Write an incomplete log line.
	writer.Write([]byte("partial line"))
	// Without a newline, DumpN should yield nil.
	if entries := rb.DumpN(10); entries != nil {
		t.Fatalf("expected no complete log line, got: %v", entries)
	}

	// Flush the writer so that the incomplete line is enqueued.
	writer.Flush()
	entries := rb.DumpN(10)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log line after flush, got %d", len(entries))
	}
	if entries[0] != "partial line" {
		t.Errorf("expected log line %q, got %q", "partial line", entries[0])
	}
}

// TestMultipleWritesForSingleLine tests that a log line built over several Write
// calls is enqueued as one complete line.
func TestMultipleWritesForSingleLine(t *testing.T) {
	rb := NewMP1COverwritingRingBuffer[string](10)
	writer := NewWriter(rb)

	// Write parts of a line.
	writer.Write([]byte("part1 "))
	writer.Write([]byte("part2"))
	// At this point, no newline has been encountered.
	if entries := rb.DumpN(10); entries != nil {
		t.Errorf("expected no complete log line, got: %v", entries)
	}

	// Write the newline to complete the log line.
	writer.Write([]byte("\n"))
	// Flush is optional here since a newline was written.
	writer.Flush()

	entries := rb.DumpN(10)
	if len(entries) != 1 {
		t.Fatalf("expected 1 complete log line, got %d", len(entries))
	}
	if entries[0] != "part1 part2" {
		t.Errorf("expected log line %q, got %q", "part1 part2", entries[0])
	}
}

// TestEdgeCases tests a couple of edge cases such as empty writes and lines starting with a newline.
func TestEdgeCases(t *testing.T) {
	// Test empty write.
	rb := NewMP1COverwritingRingBuffer[string](10)
	writer := NewWriter(rb)

	n, err := writer.Write([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error on empty write: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 bytes written on empty write, got %d", n)
	}
	if entries := rb.DumpN(10); entries != nil {
		t.Errorf("expected no log entries after empty write, got: %v", entries)
	}

	// Test a string that begins with a newline.
	writer.Write([]byte("\nfirst line\n"))
	writer.Flush()
	entries := rb.DumpN(10)
	if len(entries) != 2 {
		t.Fatalf("expected 2 log lines, got %d", len(entries))
	}
	// The first line should be empty (i.e. just "\n") and the second should be "first line\n".
	if entries[0] != "" {
		t.Errorf("expected first log line to be empty, got %q", entries[0])
	}
	if entries[1] != "first line" {
		t.Errorf("expected second log line to be \"first line\", got %q", entries[1])
	}
}
