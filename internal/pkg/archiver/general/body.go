package general

import (
	"bytes"
	"io"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/connutil"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/Zeno/pkg/models"
	warc "github.com/internetarchive/gowarc"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

func ProcessBody(u *models.URL, disableAssetsCapture, domainsCrawl bool, maxHops int, WARCTempDir string, logger *log.FieldedLogger) error {
	defer u.GetResponse().Body.Close() // Ensure the response body is closed
	// Retrieve the underlying *warc.CustomConnection if available (In unit tests, this may not be set)
	var conn *warc.CustomConnection
	bodyWithConn, ok := u.GetResponse().Body.(*connutil.BodyWithConn)
	if ok {
		conn = bodyWithConn.Conn
	} else {
		if logger != nil {
			logger.Warn("Response body is not a *BodyWithConn, connection may not be closed properly on error")
		}
	}

	return connutil.CloseConnWithError(logger, conn, processBody(u, disableAssetsCapture, domainsCrawl, maxHops, WARCTempDir))
}

// ProcessBody processes the body of a URL response, loading it into memory or a temporary file
func processBody(u *models.URL, disableAssetsCapture, domainsCrawl bool, maxHops int, WARCTempDir string) error {

	// If we are not capturing assets, not extracting outlinks, and domains crawl is disabled
	// we can just consume and discard the body
	if disableAssetsCapture && !domainsCrawl && maxHops == 0 {
		if err := connutil.CopyWithTimeout(io.Discard, u.GetResponse().Body); err != nil {
			return err
		}
	}

	buffer := new(bytes.Buffer)
	// First check HTTP Content-Type and then fallback to mimetype library.
	if u.GetMIMEType() == nil {
		// Create a buffer to hold the body (first 3KB) as suggested by mimetype author
		// https://github.com/gabriel-vasile/mimetype/blob/66e5c005d80684b64f47eeeb15ad439ee6fad667/mimetype.go#L15
		if err := connutil.CopyWithTimeoutN(buffer, u.GetResponse().Body, 3072); err != nil {
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
		if err := connutil.CopyWithTimeout(spooledBuff, u.GetResponse().Body); err != nil {
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
		if err := connutil.CopyWithTimeout(io.Discard, u.GetResponse().Body); err != nil {
			return err
		}
	}

	return nil
}
