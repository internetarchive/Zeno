package api

import (
	"net/http"

	"github.com/internetarchive/Zeno/internal/pkg/api/handlers"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
)

// registerRoutes attaches all API handlers to mux.
func registerRoutes(mux *http.ServeMux) {
	if config.Get().Prometheus {
		mux.Handle("/metrics", stats.PrometheusHandler())
	}
	mux.HandleFunc("GET /pause", handlers.GetPause)
	mux.HandleFunc("PATCH /pause", handlers.PatchPause)
}
