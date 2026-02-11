package pause

import (
	"sync"
	"sync/atomic"

	"github.com/internetarchive/Zeno/internal/pkg/stats"
)

type ControlChans struct {
	PauseCh  chan string
	ResumeCh chan struct{}
}

type pauseManager struct {
	subscribers sync.Map // Map of *ControlChans to struct{}
	isPaused    atomic.Bool
	mu          sync.RWMutex
	message     string
}

var manager = &pauseManager{}

// Subscribe returns a ControlChans struct for the subscriber to use.
// If the system is already paused, the subscriber receives an immediate pause signal.
func Subscribe() *ControlChans {
	chans := &ControlChans{
		PauseCh:  make(chan string, 1), // Buffered to ensure non-blocking sends
		ResumeCh: make(chan struct{}),  // Unbuffered, will block on send
	}
	manager.subscribers.Store(chans, struct{}{})

	// If already paused, send immediate pause signal to new subscriber
	if manager.isPaused.Load() {
		manager.mu.RLock()
		msg := manager.message
		manager.mu.RUnlock()

		select {
		case chans.PauseCh <- msg:
		default:
			// Should never happen since we just created the channel
		}
	}

	return chans
}

// Unsubscribe removes the subscriber and closes its channels.
func Unsubscribe(chans *ControlChans) {
	manager.subscribers.Delete(chans)
	// Close channels safely (deferred to avoid panic if already closed).
	defer func() {
		recover()
	}()
	close(chans.PauseCh)
	close(chans.ResumeCh)
}

// Pause sends a pause signal to all subscribers.
func Pause(message ...string) {
	swap := manager.isPaused.CompareAndSwap(false, true)
	if !swap {
		return
	}

	msg := "Paused"
	if len(message) > 0 {
		msg = message[0]
	}

	manager.mu.Lock()
	manager.message = msg
	manager.mu.Unlock()

	manager.subscribers.Range(func(key, _ any) bool {
		chans := key.(*ControlChans)
		// Send pause signal with message (non-blocking since PauseCh is buffered).
		select {
		case chans.PauseCh <- msg:
			// Signal sent.
		default:
			// PauseCh already has a signal.
		}
		return true
	})
	stats.PausedSet()
}

// Resume reads from each subscriber's ResumeCh to unblock them.
func Resume() {
	var wg sync.WaitGroup
	manager.subscribers.Range(func(key, _ any) bool {
		chans := key.(*ControlChans)
		wg.Add(1)
		go func(chans *ControlChans) {
			defer wg.Done()
			// Read from ResumeCh to unblock subscriber.
			_, ok := <-chans.ResumeCh
			if !ok {
				// Channel closed; subscriber may have unsubscribed.
				return
			}
		}(chans)
		return true
	})
	// Wait for all subscribers to send on their ResumeCh.
	wg.Wait()

	swap := manager.isPaused.CompareAndSwap(true, false)
	if !swap {
		return
	}

	manager.mu.Lock()
	manager.message = ""
	manager.mu.Unlock()

	stats.PausedReset()
}

func IsPaused() bool {
	return manager.isPaused.Load()
}

func GetMessage() string {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.message
}
