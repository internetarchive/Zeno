package frontier

import (
	"net/http"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// Handles WebSocket connections for streaming the frontier
func Handler(w http.ResponseWriter, r *http.Request) {
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
		return
	}
	defer conn.Close()

	hub := GetHub()
	poller := GetPoller()

	// Register client and ensure poller is running.
	ch := hub.Register()
	if hub.ClientCount() == 1 {
		poller.Start()
	}

	defer func() {
		hub.Unregister(ch)
		if hub.ClientCount() == 0 {
			poller.Stop()
		}
	}()

	// Writer goroutine: reads from client channel and writes to WebSocket.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for data := range ch {
			err := wsutil.WriteServerMessage(conn, ws.OpText, data)
			if err != nil {
				return
			}
		}
	}()

	// Reader goroutine: detect client close.
	go func() {
		for {
			_, _, err := wsutil.ReadClientData(conn)
			if err != nil {
				conn.Close()
				return
			}
		}
	}()

	<-done
}
