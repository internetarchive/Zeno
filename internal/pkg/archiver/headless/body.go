package headless

import (
	"bytes"
	"net/http"
	"sync"

	"github.com/go-rod/rod"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/connutil"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	warc "github.com/internetarchive/gowarc"
)

// headlessBodyBufPool pools bytes.Buffer instances for headless body processing
// to reduce allocations when processing many responses.
var headlessBodyBufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

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
	// Get a buffer from the pool
	buffer := headlessBodyBufPool.Get().(*bytes.Buffer)
	buffer.Reset()

	// Copy with timeout to the buffer
	if err := connutil.CopyWithTimeout(buffer, u.Body); err != nil {
		headlessBodyBufPool.Put(buffer)
		bodyLogger.Error("failed to copy response body", "error", err)
		return nil, err
	}

	// Make a copy of the bytes since we're returning the buffer to the pool
	result := make([]byte, buffer.Len())
	copy(result, buffer.Bytes())
	headlessBodyBufPool.Put(buffer)

	return result, nil
}
