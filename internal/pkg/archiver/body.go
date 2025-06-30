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

var errFromConn = errors.New("this error is from the connection")

// closeConnWithError call conn.CloseWithError(err) if conn is not nil and err is not nil and not equal to errFromConn.
// it always returns original error.
func closeConnWithError(conn *warc.CustomConnection, err error) error {
	if conn != nil && err != nil && !errors.Is(err, errFromConn) { // Avoid closing the connection twice if the error is from the connection itself
		if closeErr := conn.CloseWithError(err); closeErr != nil {
			if logger != nil {
				logger.Error("Failed to close connection with error", "originalErr", err, "closeErr", closeErr)
			}
		}
	}
	return err
}
func ProcessBody(u *models.URL, disableAssetsCapture, domainsCrawl bool, maxHops int, WARCTempDir string) error {
	defer u.GetResponse().Body.Close() // Ensure the response body is closed
	// Retrieve the underlying *warc.CustomConnection if available (In unit tests, this may not be set)
	var conn *warc.CustomConnection
	bodyWithConn, ok := u.GetResponse().Body.(*BodyWithConn)
	if ok {
		conn = bodyWithConn.Conn
	} else {
		if logger != nil {
			logger.Warn("Response body is not a *BodyWithConn, connection may not be closed properly on error")
		}
	}

	return closeConnWithError(conn, processBody(u, disableAssetsCapture, domainsCrawl, maxHops, WARCTempDir))
}

// ProcessBody processes the body of a URL response, loading it into memory or a temporary file
func processBody(u *models.URL, disableAssetsCapture, domainsCrawl bool, maxHops int, WARCTempDir string) error {

	// If we are not capturing assets, not extracting outlinks, and domains crawl is disabled
	// we can just consume and discard the body
	if disableAssetsCapture && !domainsCrawl && maxHops == 0 {
		if err := copyWithTimeout(io.Discard, u.GetResponse().Body); err != nil {
			return err
		}
	}

	buffer := new(bytes.Buffer)
	// First check HTTP Content-Type and then fallback to mimetype library.
	if u.GetMIMEType() == nil {
		// Create a buffer to hold the body (first 3KB) as suggested by mimetype author
		// https://github.com/gabriel-vasile/mimetype/blob/66e5c005d80684b64f47eeeb15ad439ee6fad667/mimetype.go#L15
		if err := copyWithTimeoutN(buffer, u.GetResponse().Body, 3072); err != nil {
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
		if err := copyWithTimeout(spooledBuff, u.GetResponse().Body); err != nil {
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
		if err := copyWithTimeout(io.Discard, u.GetResponse().Body); err != nil {
			return err
		}
	}

	return nil
}

// copyWithTimeout copies data and resets the read deadline after each successful read.
// NOTE: read deadline is handled by warc.CustomConnection in the background automatically
func copyWithTimeout(dst io.Writer, src io.Reader) error {
	buf := make([]byte, 4096)
	copied, maxContentLengthMiB := int64(0), config.Get().MaxContentLengthMiB
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			copied += int64(n)
			if maxContentLengthMiB > 0 && copied > int64(maxContentLengthMiB)*1024*1024 {
				return fmt.Errorf("%w: copied %d bytes, max-content-length is %d MiB", errors.New(contentlength.ContentLengthExceeded), copied, maxContentLengthMiB)
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return errors.Join(errFromConn, err)
		}
	}
	return nil
}

// copyWithTimeoutN copies a limited number of bytes and applies the timeout
// NOTE: read deadline is handled by warc.CustomConnection in the background automatically
func copyWithTimeoutN(dst io.Writer, src io.Reader, n int64) error {
	_, err := io.CopyN(dst, src, n)
	if err != nil && err != io.EOF {
		return errors.Join(errFromConn, err)
	}
	return nil
}
