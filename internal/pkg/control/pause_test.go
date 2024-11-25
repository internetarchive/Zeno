package control

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBasicFunctionality(t *testing.T) {
	// Ensure the system is not paused initially
	atomic.StoreUint32(&manager.paused, 0)

	if IsPaused() {
		t.Error("Expected IsPaused() to be false initially")
	}

	// Pause the system
	Pause()
	if !IsPaused() {
		t.Error("Expected IsPaused() to be true after Pause()")
	}

	// Resume the system
	Resume()
	if IsPaused() {
		t.Error("Expected IsPaused() to be false after Resume()")
	}
}

func TestSubscribeUnsubscribe(t *testing.T) {
	// Reset the state
	atomic.StoreUint32(&manager.paused, 0)

	ch := Subscribe()
	defer Unsubscribe(ch)

	// Read the initial state event
	select {
	case event := <-ch:
		if event != ResumeEvent {
			t.Errorf("Expected initial event to be ResumeEvent, got %v", event)
		}
	default:
		t.Error("Expected to receive initial state event")
	}

	// Pause the system and check for the event
	Pause()
	select {
	case event := <-ch:
		if event != PauseEvent {
			t.Errorf("Expected PauseEvent, got %v", event)
		}
	default:
		t.Error("Expected to receive PauseEvent")
	}

	// Resume the system and check for the event
	Resume()
	select {
	case event := <-ch:
		if event != ResumeEvent {
			t.Errorf("Expected ResumeEvent, got %v", event)
		}
	default:
		t.Error("Expected to receive ResumeEvent")
	}

	// Unsubscribe and ensure no more events are received
	Unsubscribe(ch)

	// Attempt to read from the channel; it should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("Expected channel to be closed after Unsubscribe")
		}
	default:
		t.Error("Expected channel to be closed after Unsubscribe")
	}
}

func TestMultipleSubscribers(t *testing.T) {
	// Reset the state
	atomic.StoreUint32(&manager.paused, 0)

	const numSubscribers = 10
	subs := make([]<-chan Event, numSubscribers)

	// Subscribe multiple subscribers
	for i := 0; i < numSubscribers; i++ {
		ch := Subscribe()
		subs[i] = ch
		defer Unsubscribe(ch)

		// Read the initial state event
		select {
		case event := <-ch:
			if event != ResumeEvent {
				t.Errorf("Subscriber %d: Expected initial event to be ResumeEvent, got %v", i, event)
			}
		default:
			t.Errorf("Subscriber %d: Expected to receive initial state event", i)
		}
	}

	// Pause the system
	Pause()

	// Check that all subscribers received the PauseEvent
	for i, ch := range subs {
		select {
		case event := <-ch:
			if event != PauseEvent {
				t.Errorf("Subscriber %d: Expected PauseEvent, got %v", i, event)
			}
		default:
			t.Errorf("Subscriber %d: Expected to receive PauseEvent", i)
		}
	}

	// Resume the system
	Resume()

	// Check that all subscribers received the ResumeEvent
	for i, ch := range subs {
		select {
		case event := <-ch:
			if event != ResumeEvent {
				t.Errorf("Subscriber %d: Expected ResumeEvent, got %v", i, event)
			}
		default:
			t.Errorf("Subscriber %d: Expected to receive ResumeEvent", i)
		}
	}
}

func TestConcurrentPauseResume(t *testing.T) {
	const numGoroutines = 50
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				if j%2 == 0 {
					Pause()
				} else {
					Resume()
				}
			}
		}(i)
	}

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Test completed
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out; possible deadlock or issue with concurrent Pause/Resume")
	}

	// Ensure the system is in a consistent state
	if IsPaused() {
		t.Log("System is paused at the end of TestConcurrentPauseResume")
	} else {
		t.Log("System is not paused at the end of TestConcurrentPauseResume")
	}
}

func TestWaitIfPaused(t *testing.T) {
	// Ensure the system is paused before calling WaitIfPaused()
	atomic.StoreUint32(&manager.paused, 1) // Set paused state to true

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		WaitIfPaused()
		// Indicate that WaitIfPaused() has returned
	}()

	time.Sleep(100 * time.Millisecond) // Give some time for WaitIfPaused() to block

	// Now resume the system
	Resume()

	// Wait for the goroutine to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Expected behavior
	case <-time.After(2 * time.Second):
		t.Error("WaitIfPaused() did not return after Resume()")
	}
}

func TestEdgeCases(t *testing.T) {
	const numSubscribers = 100
	subs := make([]<-chan Event, numSubscribers)

	// Subscribe multiple subscribers
	for i := 0; i < numSubscribers; i++ {
		ch := Subscribe()
		subs[i] = ch

		// Read the initial state event
		select {
		case event := <-ch:
			// We accept both ResumeEvent and PauseEvent depending on current state
			if event != ResumeEvent && event != PauseEvent {
				t.Errorf("Subscriber %d: Expected initial event to be ResumeEvent or PauseEvent, got %v", i, event)
			}
		default:
			t.Errorf("Subscriber %d: Expected to receive initial state event", i)
		}
	}

	// Rapid pause/resume cycles
	for i := 0; i < 50; i++ {
		Pause()
		Resume()
	}

	// Unsubscribe half of the subscribers during notification
	for i := 0; i < numSubscribers/2; i++ {
		Unsubscribe(subs[i])
	}

	// Pause the system
	Pause()

	// Check that remaining subscribers receive the PauseEvent
	for i := numSubscribers / 2; i < numSubscribers; i++ {
		ch := subs[i]
		select {
		case event := <-ch:
			if event != PauseEvent {
				t.Errorf("Subscriber %d: Expected PauseEvent, got %v", i, event)
			}
		default:
			t.Errorf("Subscriber %d: Expected to receive PauseEvent", i)
		}
	}
}

func TestChannelClosure(t *testing.T) {
	ch := Subscribe()

	// Read the initial event
	select {
	case <-ch:
		// Initial event received
	default:
		t.Error("Expected to receive initial state event")
	}

	Unsubscribe(ch)

	// Check that the channel is closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("Expected channel to be closed after Unsubscribe")
		}
	default:
		t.Error("Expected channel to be closed after Unsubscribe")
	}
}
