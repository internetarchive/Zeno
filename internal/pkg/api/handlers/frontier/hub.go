// Package frontier provides WebSocket streaming of reactor item deltas.
package frontier

import (
	"context"
	"sync"
)

// Hub manages WebSocket client channels for broadcasting.
type Hub struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}

	// Broadcaster channel
	broadcastCh chan []byte

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Creates a new Hub instance and starts the broadcaster goroutine.
func NewHub() *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Hub{
		clients:     make(map[chan []byte]struct{}),
		broadcastCh: make(chan []byte, 64),
		ctx:         ctx,
		cancel:      cancel,
	}
	h.wg.Add(1)
	go h.runBroadcaster()
	return h
}

// Adds a new client channel to the hub
func (h *Hub) Register() chan []byte {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Removes a client channel from the hub and closes it.
func (h *Hub) Unregister(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

// Returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Send queues data to be broadcast to all clients (non-blocking).
func (h *Hub) Send(data []byte) {
	select {
	case h.broadcastCh <- data:
	default:
		// Channel full; drop message to avoid blocking.
	}
}

// Reads from broadcastCh and fans out to all client channels
func (h *Hub) runBroadcaster() {
	defer h.wg.Done()
	for {
		select {
		case <-h.ctx.Done():
			return
		case data, ok := <-h.broadcastCh:
			if !ok {
				return
			}
			h.broadcast(data)
		}
	}
}

// Sends data to all connected clients (non-blocking).
func (h *Hub) broadcast(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- data:
		default:
			// Client channel full; skip to avoid blocking.
		}
	}
}

// Shuts down the hub and its broadcaster(s)
func (h *Hub) Close() {
	h.cancel()
	close(h.broadcastCh)
	h.wg.Wait()
}

// Singleton instance of the hub
var globalHub = NewHub()

// Returns the global hub instance
func GetHub() *Hub {
	return globalHub
}
