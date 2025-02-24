// Package api defines the web API for Zeno.
package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
)

var (
	server *http.Server
	once   sync.Once
	// ErrAPIAlreadyInitialized is returned when the API server is already initialized.
	ErrAPIAlreadyInitialized = errors.New("API server already initialized")
)

// Start begins serving HTTP requests in a separate goroutine.
func Start() error {
	var done bool

	once.Do(func() {
		mux := http.NewServeMux()

		if config.Get().Prometheus {
			mux.Handle("/metrics", stats.PromHandler())
		}

		server = &http.Server{
			Addr:    ":" + config.Get().APIPort,
			Handler: mux,
		}

		go func() {
			log.Printf("Starting API server on %s", server.Addr)
			// ListenAndServe returns http.ErrServerClosed when Shutdown is called.
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("ListenAndServe error: %v", err)
			}
		}()

		done = true
	})

	if !done {
		return ErrAPIAlreadyInitialized
	}

	return nil
}

// Stop gracefully shuts down the server within the provided timeout.
func Stop(timeout time.Duration) error {
	log.Printf("Stopping API server on %s", server.Addr)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return server.Shutdown(ctx)
}
