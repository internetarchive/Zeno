package control

import (
	"sync"
	"sync/atomic"
)

// subscriber represents a goroutine subscribed to pause/resume events.
type subscriber struct {
	ch chan Event
}

// pauseManager manages the paused state and subscribers.
type pauseManager struct {
	paused      uint32
	subscribers sync.Map // Map of *subscriber to struct{}
}

var manager = &pauseManager{}

// IsPaused returns true if the system is currently paused.
func IsPaused() bool {
	return atomic.LoadUint32(&manager.paused) == 1
}

// Pause sets the paused state to true and notifies all subscribers.
func Pause() {
	if atomic.CompareAndSwapUint32(&manager.paused, 0, 1) {
		manager.notifySubscribers(PauseEvent)
	}
}

// Resume sets the paused state to false and notifies all subscribers.
func Resume() {
	if atomic.CompareAndSwapUint32(&manager.paused, 1, 0) {
		manager.notifySubscribers(ResumeEvent)
	}
}

// Subscribe returns a channel to receive pause and resume events.
func Subscribe() <-chan Event {
	sub := &subscriber{
		ch: make(chan Event, 1),
	}

	// Store the subscriber in the sync.Map
	manager.subscribers.Store(sub, struct{}{})

	// Send the current state immediately to the subscriber.
	if IsPaused() {
		sub.ch <- PauseEvent
	} else {
		sub.ch <- ResumeEvent
	}

	return sub.ch
}

// Unsubscribe removes a subscriber from the list.
func Unsubscribe(ch <-chan Event) {
	// Find and delete the subscriber
	manager.subscribers.Range(func(key, _ interface{}) bool {
		sub := key.(*subscriber)
		if sub.ch == ch {
			manager.subscribers.Delete(sub)
			close(sub.ch)
			return false // Stop iterating
		}
		return true // Continue iterating
	})
}

// notifySubscribers sends an event to all subscribers.
func (m *pauseManager) notifySubscribers(event Event) {
	m.subscribers.Range(func(key, _ interface{}) bool {
		sub := key.(*subscriber)
		select {
		case sub.ch <- event:
			// Event sent successfully
		default:
			// Subscriber's channel is full; skip to prevent blocking
		}
		return true // Continue iterating
	})
}

// WaitIfPaused blocks the caller if the system is paused until it is resumed.
func WaitIfPaused() {
	if IsPaused() {
		ch := Subscribe()
		defer Unsubscribe(ch)

		for event := range ch {
			if event == ResumeEvent {
				break
			}
		}
	}
}
