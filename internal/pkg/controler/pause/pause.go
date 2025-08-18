package pause

import (
	"sync"
	"sync/atomic"

	"github.com/internetarchive/Zeno/internal/pkg/stats"
)

type ControlChans struct {
	PauseCh  chan struct{}
	ResumeCh chan struct{}
}

type pauseManager struct {
	subscribers sync.Map // Map of *ControlChans to struct{}
	isPaused    atomic.Bool
	message     string
}

var manager = &pauseManager{}

// Subscribe returns a ControlChans struct for the subscriber to use.
func Subscribe() *ControlChans {
	chans := &ControlChans{
		PauseCh:  make(chan struct{}, 1), // Buffered to ensure non-blocking sends
		ResumeCh: make(chan struct{}),    // Unbuffered, will block on send
	}
	manager.subscribers.Store(chans, struct{}{})
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

	if len(message) == 0 {
		message = append(message, "Paused")
	}

	manager.message = message[0]

	manager.subscribers.Range(func(key, _ any) bool {
		chans := key.(*ControlChans)
		// Send pause signal (non-blocking since PauseCh is buffered).
		select {
		case chans.PauseCh <- struct{}{}:
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
	manager.message = ""

	stats.PausedReset()
}

func IsPaused() bool {
	return manager.isPaused.Load()
}

func GetMessage() string {
	return manager.message
}
