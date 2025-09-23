package api

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
)

// StatusResponse represents the structure of the status API response
// similar to warcprox's status endpoint
type StatusResponse struct {
	Role             string         `json:"role"`
	Version          string         `json:"version"`
	Host             string         `json:"host"`
	URLsProcessed    int64          `json:"urls_processed"`
	WARCBytesWritten int64          `json:"warc_bytes_written"`
	StartTime        string         `json:"start_time"`
	QueuedURLs       int            `json:"queued_urls"`
	ComponentQueues  map[string]int `json:"component_queues"`
	Stats            map[string]any `json:"stats"`
}

var startTime = time.Now()

// statusHandler handles GET requests to /status
func statusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Get version
	version := utils.GetVersion()

	// Get all stats
	allStats := stats.GetMapTUI()

	// Get channel queue sizes
	channelQueues := stats.GetChannelQueueSizes()

	// Calculate total queued URLs
	totalQueued := 0
	for _, size := range channelQueues {
		totalQueued += size
	}

	// Build response
	response := StatusResponse{
		Role:             "zeno",
		Version:          version.Version,
		Host:             hostname,
		URLsProcessed:    getInt64FromStats(allStats, "Total URL crawled"),
		WARCBytesWritten: getInt64FromStats(allStats, "WARC data total (GB)") * 1e9, // Convert back to bytes
		StartTime:        startTime.Format(time.RFC3339),
		QueuedURLs:       totalQueued,
		ComponentQueues:  channelQueues,
		Stats:            allStats,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
		return
	}
}

// Helper function to safely get int64 values from stats map
func getInt64FromStats(stats map[string]any, key string) int64 {
	if val, ok := stats[key]; ok {
		switch v := val.(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case float64:
			return int64(v)
		}
	}
	return 0
}
