package controler

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBasicPauseResume(t *testing.T) {
	manager = &pauseManager{}

	var wg sync.WaitGroup
	wg.Add(1)

	subscribed := make(chan struct{})
	pausedCh := make(chan struct{})
	resumedCh := make(chan struct{})

	go func() {
		defer wg.Done()
		controlChans := Subscribe()
		defer Unsubscribe(controlChans)

		subscribed <- struct{}{}

		for {
			select {
			case <-controlChans.PauseCh:
				// Signal that we have received the pause signal
				pausedCh <- struct{}{}
				// Attempt to send to ResumeCh; blocks until Resume() reads from it.
				controlChans.ResumeCh <- struct{}{}
				// Signal that we have resumed
				resumedCh <- struct{}{}
				return // Exit after resuming.
			default:
				time.Sleep(10 * time.Millisecond) // Simulate work.
			}
		}
	}()

	// Wait for the goroutine to subscribe
	<-subscribed

	// Pause the system.
	Pause()

	// Wait for the goroutine to signal that it has paused
	select {
	case <-pausedCh:
		// Paused successfully
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Subscriber did not receive pause signal")
	}

	// Resume the system.
	Resume()

	// Wait for the goroutine to signal that it has resumed
	select {
	case <-resumedCh:
		// Resumed successfully
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Subscriber did not resume")
	}

	wg.Wait()
}

func TestMultipleSubscribers(t *testing.T) {
	manager = &pauseManager{}
	const numSubscribers = 10
	var wg sync.WaitGroup

	subscribedChans := make([]chan struct{}, numSubscribers)
	pausedChans := make([]chan struct{}, numSubscribers)
	resumedChans := make([]chan struct{}, numSubscribers)

	// Create multiple subscribers.
	for i := 0; i < numSubscribers; i++ {
		wg.Add(1)
		subscribedChans[i] = make(chan struct{})
		pausedChans[i] = make(chan struct{})
		resumedChans[i] = make(chan struct{})

		go func(idx int) {
			defer wg.Done()
			controlChans := Subscribe()
			defer Unsubscribe(controlChans)

			subscribedChans[idx] <- struct{}{}

			for {
				select {
				case <-controlChans.PauseCh:
					// Signal that we have paused
					pausedChans[idx] <- struct{}{}
					// Attempt to send to ResumeCh; blocks until Resume() reads from it.
					controlChans.ResumeCh <- struct{}{}
					// Signal that we have resumed
					resumedChans[idx] <- struct{}{}
					return // Exit after resuming.
				default:
					time.Sleep(10 * time.Millisecond) // Simulate work.
				}
			}
		}(i)
	}

	// Wait for all subscribers to subscribe
	for i := 0; i < numSubscribers; i++ {
		<-subscribedChans[i]
	}

	// Pause the system.
	Pause()

	// Wait for all subscribers to acknowledge the pause
	for i := 0; i < numSubscribers; i++ {
		select {
		case <-pausedChans[i]:
			// Subscriber paused
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("Subscriber %d did not receive pause signal", i)
		}
	}

	// Resume the system.
	Resume()

	// Wait for all subscribers to acknowledge the resume
	for i := 0; i < numSubscribers; i++ {
		select {
		case <-resumedChans[i]:
			// Subscriber resumed
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("Subscriber %d did not resume", i)
		}
	}

	wg.Wait()
}

func TestSubscriberUnsubscribeDuringPause(t *testing.T) {
	manager = &pauseManager{}
	var wg sync.WaitGroup
	wg.Add(1)

	subscribedCh := make(chan struct{})
	pausedCh := make(chan struct{})

	go func() {
		defer wg.Done()
		controlChans := Subscribe()
		defer Unsubscribe(controlChans)

		subscribedCh <- struct{}{}

		for {
			select {
			case <-controlChans.PauseCh:
				// Signal that we have paused
				pausedCh <- struct{}{}
				// Unsubscribe during pause.
				Unsubscribe(controlChans)
				return
			default:
				time.Sleep(10 * time.Millisecond) // Simulate work.
			}
		}
	}()

	// Wait for the subscriber to subscribe
	<-subscribedCh

	// Pause the system.
	Pause()

	// Wait for the subscriber to acknowledge the pause
	select {
	case <-pausedCh:
		// Subscriber paused and unsubscribed
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Subscriber did not receive pause signal")
	}

	// Resume the system.
	Resume()
	time.Sleep(100 * time.Millisecond) // Allow any processing.

	wg.Wait()
}

func TestConcurrentPauseResume(t *testing.T) {
	manager = &pauseManager{}
	const numSubscribers = 5
	const numCycles = 10

	var wg sync.WaitGroup
	wg.Add(numSubscribers)

	// Channels to signal pause and resume completions
	subscribedCh := make(chan struct{})
	pauseComplete := make(chan struct{})
	resumeComplete := make(chan struct{})

	// Channel to receive counts from goroutines
	countsCh := make(chan struct {
		pauses  int32
		resumes int32
	}, numSubscribers)

	// Create subscribers
	for i := 0; i < numSubscribers; i++ {
		go func() {
			defer wg.Done()
			controlChans := Subscribe()
			defer Unsubscribe(controlChans)

			subscribedCh <- struct{}{}

			var pauses, resumes int32

			for j := 0; j < numCycles; j++ {
				// Wait for pause signal
				<-controlChans.PauseCh
				pauses++

				// Signal that we've received the pause
				pauseComplete <- struct{}{}

				// Block until resumed
				controlChans.ResumeCh <- struct{}{}
				resumes++

				// Signal that we've resumed
				resumeComplete <- struct{}{}
			}

			// Send counts back to main goroutine
			countsCh <- struct {
				pauses  int32
				resumes int32
			}{pauses, resumes}
		}()
	}

	// Wait for all subscribers to subscribe
	for i := 0; i < numSubscribers; i++ {
		<-subscribedCh
	}

	// Perform pause and resume cycles
	for i := 0; i < numCycles; i++ {
		// Perform pause
		Pause()

		// Wait for all subscribers to acknowledge the pause
		for j := 0; j < numSubscribers; j++ {
			<-pauseComplete
		}

		// Perform resume
		Resume()

		// Wait for all subscribers to acknowledge the resume
		for j := 0; j < numSubscribers; j++ {
			<-resumeComplete
		}
	}

	// Wait for all subscribers to finish
	wg.Wait()
	close(countsCh)

	// Verify that all subscribers have processed the correct number of pauses and resumes
	for counts := range countsCh {
		if counts.pauses != numCycles {
			t.Fatalf("Subscriber expected to process %d pauses, but processed %d", numCycles, counts.pauses)
		}
		if counts.resumes != numCycles {
			t.Fatalf("Subscriber expected to process %d resumes, but processed %d", numCycles, counts.resumes)
		}
	}
}

func TestPauseResumeWithUnsubscribe(t *testing.T) {
	manager = &pauseManager{}
	var wg sync.WaitGroup
	wg.Add(1)

	subscribedCh := make(chan struct{})
	pausedCh := make(chan struct{})
	resumedCh := make(chan struct{})

	go func() {
		defer wg.Done()
		controlChans := Subscribe()
		subscribedCh <- struct{}{}
		// Unsubscribe after resuming.

		for {
			select {
			case <-controlChans.PauseCh:
				// Signal that we have paused
				pausedCh <- struct{}{}
				// Attempt to send to ResumeCh; blocks until Resume() reads from it.
				controlChans.ResumeCh <- struct{}{}
				// Signal that we have resumed
				resumedCh <- struct{}{}
				// Unsubscribe after resuming.
				Unsubscribe(controlChans)
				return
			default:
				time.Sleep(10 * time.Millisecond) // Simulate work.
			}
		}
	}()

	// Wait for the subscriber to subscribe
	<-subscribedCh

	// Pause the system.
	Pause()

	// Wait for the subscriber to acknowledge pause
	select {
	case <-pausedCh:
		// Subscriber paused
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Subscriber did not receive pause signal")
	}

	// Resume the system.
	Resume()

	// Wait for the subscriber to acknowledge resume
	select {
	case <-resumedCh:
		// Subscriber resumed
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Subscriber did not resume")
	}

	wg.Wait()
}

func TestNoSubscribers(t *testing.T) {
	manager = &pauseManager{}
	// Call Pause() and Resume() when there are no subscribers.
	// If no panic occurs, the test passes.
	Pause()
	Resume()
}

func TestPauseResumeE2E(t *testing.T) {
	manager = &pauseManager{}
	var workCounter int32 // Counts the amount of work done.
	var wg sync.WaitGroup
	wg.Add(1)

	ctx, cancel := context.WithCancel(context.Background())

	// Start the worker goroutine.
	go func() {
		controlChans := Subscribe()
		defer Unsubscribe(controlChans)
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-controlChans.PauseCh:
				// Attempt to send to ResumeCh; blocks until Resume() reads from it.
				controlChans.ResumeCh <- struct{}{}
			default:
				// Simulate work.
				atomic.AddInt32(&workCounter, 1)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	// Allow the worker to do some work.
	time.Sleep(1 * time.Second)
	workBeforePause := atomic.LoadInt32(&workCounter)

	// Pause the system.
	Pause()
	pauseStart := time.Now()

	// Sleep for 1 second to keep the system paused.
	time.Sleep(1 * time.Second)

	// Resume the system.
	Resume()
	pauseDuration := time.Since(pauseStart)

	// Allow the worker to do more work.
	time.Sleep(1 * time.Second)
	workAfterResume := atomic.LoadInt32(&workCounter)

	// Calculate the amount of work done during the pause.
	workDuringPause := workAfterResume - workBeforePause - 10 // Expected 10 units of work after resume.

	// Check that no work was done during the pause.
	if workDuringPause != 0 {
		t.Fatalf("Expected no work during pause, but got %d units of work", workDuringPause)
	}

	// Verify that the pause duration is approximately 1 second.
	if pauseDuration < 900*time.Millisecond || pauseDuration > 1100*time.Millisecond {
		t.Fatalf("Expected pause duration around 1 second, but got %v", pauseDuration)
	}

	cancel()
	wg.Wait()
}
