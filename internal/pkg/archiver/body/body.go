package body

import (
	"io"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/config"
)

// CopyWithTimeout copies data and resets the read deadline after each successful read
func CopyWithTimeout(dst io.Writer, src io.Reader, conn interface{ SetReadDeadline(time.Time) error }) error {
	buf := make([]byte, 4096)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			// Reset the deadline after each successful read
			if conn != nil {
				err = conn.SetReadDeadline(time.Now().Add(time.Duration(config.Get().HTTPReadDeadline)))
				if err != nil {
					return err
				}
			}
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	return nil
}

// CopyWithTimeoutN copies a limited number of bytes and applies the timeout
func CopyWithTimeoutN(dst io.Writer, src io.Reader, n int64, conn interface{ SetReadDeadline(time.Time) error }) error {
	_, err := io.CopyN(dst, src, n)
	if err != nil && err != io.EOF {
		return err
	}

	// Reset deadline after partial read
	if conn != nil {
		err = conn.SetReadDeadline(time.Now().Add(time.Duration(config.Get().HTTPReadDeadline)))
		if err != nil {
			return err
		}
	}
	return nil
}
