package ringbuffer

import "bytes"

// Writer implements io.Writer and writes complete log lines to a ring buffer.
// It accumulates partial writes until a newline is seen.
type Writer struct {
	rb  *MP1COverwritingRingBuffer[string]
	buf []byte
}

// NewWriter returns a new Writer backed by the given ring buffer.
func NewWriter(rb *MP1COverwritingRingBuffer[string]) *Writer {
	return &Writer{
		rb: rb,
	}
}

// Write implements io.Writer.
// It scans the input for newline characters, enqueuing each complete log line into the ring buffer.
// Any bytes after the last newline remain buffered until the next Write.
func (w *Writer) Write(p []byte) (n int, err error) {
	n = len(p)
	// Append new bytes to our internal buffer.
	w.buf = append(w.buf, p...)

	// Process any complete lines.
	for {
		// Find the index of the newline character.
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			// No newline found: leave any incomplete log line in the buffer.
			break
		}
		// Extract a complete log line (not including the newline).
		line := string(w.buf[:idx])
		// Enqueue the complete log line into the ring buffer.
		w.rb.Enqueue(line)
		// Remove the processed log line (and it's newline) from the buffer.
		w.buf = w.buf[idx+1:]
	}
	return n, nil
}

// Flush force writing any buffered data (even if incomplete).
// This is not part of io.Writer, but can be useful in some logging setups.
func (w *Writer) Flush() error {
	if len(w.buf) > 0 {
		// Optionally, decide how to handle an incomplete log line.
		// Here we enqueue it as is.
		w.rb.Enqueue(string(w.buf))
		w.buf = nil
	}
	return nil
}
