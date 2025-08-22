package headless

import (
	"bytes"
	"net/http"

	"github.com/go-rod/rod"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/connutil"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	warc "github.com/internetarchive/gowarc"
)

var bodyLogger = log.NewFieldedLogger(&log.Fields{
	"component": "archiver.headless.body.process",
})

// ProcessBody processes the body of a URL response, loading it into memory or a temporary file
func ProcessBodyHeadless(hijack *rod.Hijack, u *http.Response) ([]byte, error) {
	defer u.Body.Close() // Ensure the response body is closed

	// Retrieve the underlying *warc.CustomConnection if available (In unit tests, this may not be set)
	var conn *warc.CustomConnection
	bodyWithConn, ok := u.Body.(*connutil.BodyWithConn)
	if ok {
		conn = bodyWithConn.Conn
	} else {
		bodyLogger.Warn("Response body is not a *BodyWithConn, connection may not be closed properly on error")
	}

	fullBody, err := processBodyHeadless(u)
	return fullBody, connutil.CloseConnWithError(bodyLogger, conn, err)
}

func processBodyHeadless(u *http.Response) ([]byte, error) {
	// Copy with timeout to the hijack response
	buffer := new(bytes.Buffer)
	if err := connutil.CopyWithTimeout(buffer, u.Body); err != nil {
		bodyLogger.Error("failed to copy response body", "error", err)
		return nil, err
	}

	return buffer.Bytes(), nil
}
