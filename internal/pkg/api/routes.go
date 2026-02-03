package api

import (
	"net/http"
	"path/filepath"

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

	// Other API routes take precedence
	if dir := config.Get().APIStaticDir; dir != "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			abs = dir
		}
		mux.Handle("/", http.FileServer(http.Dir(abs)))
	}
}
