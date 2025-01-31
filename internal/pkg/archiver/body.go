package archiver

import (
	"bytes"
	"io"
	"strings"
	"time"

	"github.com/CorentinB/warc/pkg/spooledtempfile"
	"github.com/gabriel-vasile/mimetype"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/pkg/models"
)

// ProcessBody processes the body of a URL response, loading it into memory or a temporary file
func ProcessBody(u *models.URL, disableAssetsCapture, domainsCrawl bool, maxHops int, WARCTempDir string) error {
	defer u.GetResponse().Body.Close() // Ensure the response body is closed

	// Retrieve the underlying TCP connection and apply a 10s read deadline
	conn, ok := u.GetResponse().Body.(interface{ SetReadDeadline(time.Time) error })
	if ok {
		conn.SetReadDeadline(time.Now().Add(time.Duration(config.Get().HTTPReadDeadline)))
	}

	// If we are not capturing assets, not extracting outlinks, and domains crawl is disabled
	// we can just consume and discard the body
	if disableAssetsCapture && !domainsCrawl && maxHops == 0 {
		if err := copyWithTimeout(io.Discard, u.GetResponse().Body, conn); err != nil {
			return err
		}
	}

	// Create a buffer to hold the body (first 2KB)
	buffer := new(bytes.Buffer)
	if err := copyWithTimeoutN(buffer, u.GetResponse().Body, 2048, conn); err != nil {
		return err
	}

	// Detect and set MIME type
	u.SetMIMEType(mimetype.Detect(buffer.Bytes()))

	// Check if the MIME type requires post-processing
	if (u.GetMIMEType().Parent() != nil && u.GetMIMEType().Parent().String() == "text/plain") ||
		strings.Contains(u.GetMIMEType().String(), "text/") {

		// Create a temp file with a 2MB memory buffer
		spooledBuff := spooledtempfile.NewSpooledTempFile("zeno", WARCTempDir, 2097152, false, -1)
		_, err := io.Copy(spooledBuff, buffer)
		if err != nil {
			closeErr := spooledBuff.Close()
			if closeErr != nil {
				panic(closeErr)
			}
			return err
		}

		// Read the rest of the body into the spooled buffer
		if err := copyWithTimeout(spooledBuff, u.GetResponse().Body, conn); err != nil {
			closeErr := spooledBuff.Close()
			if closeErr != nil {
				panic(closeErr)
			}
			return err
		}

		u.SetBody(spooledBuff)
		u.RewindBody()

		return nil
	} else {
		// Read the rest of the body but discard it
		if err := copyWithTimeout(io.Discard, u.GetResponse().Body, conn); err != nil {
			return err
		}
	}

	return nil
}

// copyWithTimeout copies data and resets the read deadline after each successful read
func copyWithTimeout(dst io.Writer, src io.Reader, conn interface{ SetReadDeadline(time.Time) error }) error {
	buf := make([]byte, 4096)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			// Reset the deadline after each successful read
			if conn != nil {
				conn.SetReadDeadline(time.Now().Add(time.Duration(config.Get().HTTPReadDeadline)))
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

// copyWithTimeoutN copies a limited number of bytes and applies the timeout
func copyWithTimeoutN(dst io.Writer, src io.Reader, n int64, conn interface{ SetReadDeadline(time.Time) error }) error {
	_, err := io.CopyN(dst, src, n)
	if err != nil && err != io.EOF {
		return err
	}
	// Reset deadline after partial read
	if conn != nil {
		conn.SetReadDeadline(time.Now().Add(time.Duration(config.Get().HTTPReadDeadline)))
	}
	return nil
}
