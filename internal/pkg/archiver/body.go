package archiver

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/discard/discarder/contentlength"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/Zeno/pkg/models"
	warc "github.com/internetarchive/gowarc"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

// BodyWithConn is a wrapper around resp.Body that also holds a reference to the warc.CustomConnection
// This is necessary to reset the read deadline after each read operation
type BodyWithConn struct {
	io.ReadCloser
	Conn *warc.CustomConnection
}

func ProcessBody(u *models.URL, disableAssetsCapture, domainsCrawl bool, maxHops int, WARCTempDir string) error {
	defer u.GetResponse().Body.Close() // Ensure the response body is closed

	// Retrieve the underlying TCP connection and apply a 10s read deadline
	conn, ok := u.GetResponse().Body.(interface{ SetReadDeadline(time.Time) error })
	if ok {
		err := conn.SetReadDeadline(time.Now().Add(time.Duration(config.Get().HTTPReadDeadline)))
		if err != nil {
			return err
		}
	}

	// If we are not capturing assets, not extracting outlinks, and domains crawl is disabled
	// we can just consume and discard the body
	if disableAssetsCapture && !domainsCrawl && maxHops == 0 {
		if err := copyWithTimeout(io.Discard, u.GetResponse().Body, conn); err != nil {
			return err
		}
	}

	buffer := new(bytes.Buffer)
	// First check HTTP Content-Type and then fallback to mimetype library.
	if u.GetMIMEType() == nil {
		// Create a buffer to hold the body (first 3KB) as suggested by mimetype author
		// https://github.com/gabriel-vasile/mimetype/blob/66e5c005d80684b64f47eeeb15ad439ee6fad667/mimetype.go#L15
		if err := copyWithTimeoutN(buffer, u.GetResponse().Body, 3072, conn); err != nil {
			return err
		}
		u.SetMIMEType(mimetype.Detect(buffer.Bytes()))
	}

	// Check if the MIME type requires post-processing
	if (u.GetMIMEType().Parent() != nil && utils.IsMIMETypeInHierarchy(u.GetMIMEType().Parent(), "text/plain")) ||
		u.GetMIMEType().Is("application/pdf") ||
		strings.Contains(u.GetMIMEType().String(), "text/") {

		// Create a temp file with a 8MB memory buffer
		spooledBuff := spooledtempfile.NewSpooledTempFile("zeno", WARCTempDir, 8000000, false, -1)
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
	copied, maxContentLengthMiB := int64(0), config.Get().MaxContentLengthMiB
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
			copied += int64(n)
			if maxContentLengthMiB > 0 && copied > int64(maxContentLengthMiB)*1024*1024 {
				return errors.New(contentlength.ContentLengthExceeded)
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
		err = conn.SetReadDeadline(time.Now().Add(time.Duration(config.Get().HTTPReadDeadline)))
		if err != nil {
			return err
		}
	}
	return nil
}
