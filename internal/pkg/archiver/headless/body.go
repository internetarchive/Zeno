package headless

import (
	"bytes"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/go-rod/rod"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/body"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
)

// ProcessBody processes the body of a URL response, loading it into memory or a temporary file
func ProcessBodyHeadless(hijack *rod.Hijack, u *http.Response) ([]byte, error) {
	defer u.Body.Close() // Ensure the response body is closed

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

	// Retrieve the underlying TCP connection and apply a 10s read deadline
	conn, ok := u.Body.(interface{ SetReadDeadline(time.Time) error })
	if ok {
		err := conn.SetReadDeadline(time.Now().Add(time.Duration(config.Get().HTTPReadDeadline)))
		if err != nil {
			logger.Error("failed to set read deadline", "error", err)
			return nil, err
		}
	}

	// Copy with timeout to the hijack response
	buffer := new(bytes.Buffer)
	if err := body.CopyWithTimeout(buffer, u.Body, conn); err != nil {
		logger.Error("failed to copy response body", "error", err)
		return nil, err
	}

	return buffer.Bytes(), nil
}
