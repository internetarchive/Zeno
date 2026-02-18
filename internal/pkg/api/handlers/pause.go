package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
)

// JSON schema
type PauseState struct {
	Paused bool `json:"paused"`
}

// GET /pause.
func GetPause(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode(PauseState{Paused: pause.IsPaused()})
}

// PATCH /pause
func PatchPause(w http.ResponseWriter, r *http.Request) {
	var state PauseState
	if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
		http.Error(w, "body must be {\"paused\": true|false}", http.StatusBadRequest)
		return
	}
	if state.Paused {
		pause.Pause()
	} else {
		pause.Resume()
	}
}
