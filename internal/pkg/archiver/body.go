package archiver

import (
	"bytes"
	"io"
	"strings"

	"github.com/CorentinB/warc"
	"github.com/gabriel-vasile/mimetype"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/pkg/models"
)

func processBody(u *models.URL) error {
	defer u.GetResponse().Body.Close() // Ensure the response body is closed

	// If we are not capturing assets nor do we want to extract outlinks (and domains crawl is disabled)
	// we can just consume the body and discard it
	if config.Get().DisableAssetsCapture && !config.Get().DomainsCrawl && config.Get().MaxHops == 0 {
		// Read the rest of the body but discard it
		_, err := io.Copy(io.Discard, u.GetResponse().Body)
		if err != nil {
			return err
		}
	}

	// Create a buffer to hold the body
	buffer := new(bytes.Buffer)
	_, err := io.CopyN(buffer, u.GetResponse().Body, 2048)
	if err != nil && err != io.EOF {
		return err
	}

	// We do not use http.DetectContentType because it only supports
	// a limited number of MIME types, those commonly found in web.
	u.SetMIMEType(mimetype.Detect(buffer.Bytes()))

	// Check if the MIME type is one that we post-process
	if (u.GetMIMEType().Parent() != nil &&
		u.GetMIMEType().Parent().String() == "text/plain") ||
		strings.Contains(u.GetMIMEType().String(), "text/") {
		// Create a spooled temp file, that is a ReadWriteSeeker that writes to a temporary file
		// when the in-memory buffer exceeds a certain size. (here, 2MB)
		spooledBuff := warc.NewSpooledTempFile("zeno", config.Get().WARCTempDir, 2097152, false)
		_, err := io.Copy(spooledBuff, buffer)
		if err != nil {
			return err
		}

		// Read the rest of the body and set it in SetBody()
		_, err = io.Copy(spooledBuff, u.GetResponse().Body)
		if err != nil && err != io.EOF {
			return err
		}

		u.SetBody(spooledBuff)
		u.RewindBody()

		return nil
	} else {
		// Read the rest of the body but discard it
		_, err := io.Copy(io.Discard, u.GetResponse().Body)
		if err != nil {
			return err
		}
	}

	return nil
}
