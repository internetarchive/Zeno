package headless

import (
	"bytes"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/go-rod/rod"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/body"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	warc "github.com/internetarchive/gowarc"
)

// ProcessBody processes the body of a URL response, loading it into memory or a temporary file
func ProcessBodyHeadless(hijack *rod.Hijack, u *http.Response) ([]byte, error) {
	defer u.Body.Close() // Ensure the response body is closed

	// Retrieve the underlying *warc.CustomConnection if available (In unit tests, this may not be set)
	var conn *warc.CustomConnection
	bodyWithConn, ok := u.Body.(*body.BodyWithConn)
	if ok {
		conn = bodyWithConn.Conn
	} else {
		logger.Warn("Response body is not a *BodyWithConn, connection may not be closed properly on error")
	}

	fullBody, err := processBodyHeadless(hijack, u)
	return fullBody, body.CloseConnWithError(logger, conn, err)
}

func processBodyHeadless(hijack *rod.Hijack, u *http.Response) ([]byte, error) {
	onExit := atomic.Bool{}
	logger := log.NewFieldedLogger(&log.Fields{
		"url": hijack.Request.Req().URL.String(),
	})
	logger.Debug("processing body for hijack request")

	go func() {
		ticker := time.NewTicker(time.Duration(time.Second))
		defer ticker.Stop()
		for range ticker.C {
			if onExit.Load() {
				return
			}
			logger.Info("resource in progress")
		}
	}()

	defer func() {
		onExit.Store(true)
	}()

	// Copy with timeout to the hijack response
	buffer := new(bytes.Buffer)
	if err := body.CopyWithTimeout(buffer, u.Body); err != nil {
		logger.Error("failed to copy response body", "error", err)
		return nil, err
	}

	return buffer.Bytes(), nil
}
