package pause

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/stats"
)

func TestBasicPauseResume(t *testing.T) {
	stats.Init()
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
	stats.Init()
	manager = &pauseManager{}
	const numSubscribers = 10
	var wg sync.WaitGroup

	subscribedChans := make([]chan struct{}, numSubscribers)
	pausedChans := make([]chan struct{}, numSubscribers)
	resumedChans := make([]chan struct{}, numSubscribers)

	// Create multiple subscribers.
	for i := range numSubscribers {
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
	for i := range numSubscribers {
		<-subscribedChans[i]
	}

	// Pause the system.
	Pause()

	// Wait for all subscribers to acknowledge the pause
	for i := range numSubscribers {
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
	for i := range numSubscribers {
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
	stats.Init()
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
	stats.Init()
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
	for range numSubscribers {
		go func() {
			defer wg.Done()
			controlChans := Subscribe()
			defer Unsubscribe(controlChans)

			subscribedCh <- struct{}{}

			var pauses, resumes int32

			for range numCycles {
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
	for range numSubscribers {
		<-subscribedCh
	}

	// Perform pause and resume cycles
	for range numCycles {
		// Perform pause
		Pause()

		// Wait for all subscribers to acknowledge the pause
		for range numSubscribers {
			<-pauseComplete
		}

		// Perform resume
		Resume()

		// Wait for all subscribers to acknowledge the resume
		for range numSubscribers {
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
	stats.Init()
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
	stats.Init()
	manager = &pauseManager{}
	// Call Pause() and Resume() when there are no subscribers.
	// If no panic occurs, the test passes.
	Pause()
	Resume()
}

func TestPauseResumeE2E(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		stats.Init()
		manager = &pauseManager{}
		var workCounter int32 // Counts the amount of work done.
		var wg sync.WaitGroup

		ctx, cancel := context.WithCancel(context.Background())

		// Start the worker goroutine.
		wg.Go(func() {
			controlChans := Subscribe()
			defer Unsubscribe(controlChans)
			for {
				select {
				case <-ctx.Done():
					return
				case <-controlChans.PauseCh:
					// Attempt to send to ResumeCh; blocks until Resume() reads from it.
					controlChans.ResumeCh <- struct{}{}
				default:
					// Simulate work.
					time.Sleep(100 * time.Millisecond)
					atomic.AddInt32(&workCounter, 1)
					t.Log(time.Now(), "Work done:", atomic.LoadInt32(&workCounter))
				}
			}
		})

		// Allow the worker to do some work.
		time.Sleep(1 * time.Second)
		synctest.Wait()

		// Pause the system.
		t.Log("Pausing...")
		Pause()
		time.Sleep(500 * time.Millisecond) // Give some time to ensure the worker has paused.
		synctest.Wait()
		t.Log("Should be paused now.")

		workAfterPause := atomic.LoadInt32(&workCounter)

		time.Sleep(100 * time.Second)
		synctest.Wait()

		// Resume the system.
		workBeforeResume := atomic.LoadInt32(&workCounter)
		t.Log("Resuming...")
		Resume()

		// Allow the worker to do more work.
		time.Sleep(1 * time.Second)
		synctest.Wait()
		t.Log("Finalizing...")
		cancel()
		wg.Wait()
		workFinal := atomic.LoadInt32(&workCounter)

		// Calculate the amount of work done during the pause.
		workDuringPause := workBeforeResume - workAfterPause

		// Check that no work was done during the pause.
		if workDuringPause != 0 {
			t.Fatalf("Expected no work during pause, but got %d units of work", workDuringPause)
		}

		t.Logf("Work done after pause: %d", workAfterPause)
		t.Logf("Work done before resume: %d", workBeforeResume)
		t.Logf("Work done after resume: %d", workFinal)
		if workFinal < 20 || workFinal > 22 {
			t.Fatalf("Expected [20, 21, 22] units of all work, but got %d", workFinal)
		}
	})
}

func TestSubscribeAfterPause(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Alice subscribes to the controller, which subsequently paused,
		// Bob subscribes to the controller after it is already paused.
		stats.Init()
		manager = &pauseManager{}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Track pause state for each listener
		var alicePaused atomic.Bool
		var bobPaused atomic.Bool
		var aliceResumed atomic.Bool
		var bobResumed atomic.Bool

		var wg sync.WaitGroup

		// Start Alice
		wg.Add(1)
		go func() {
			defer wg.Done()
			controlChans := Subscribe()
			defer Unsubscribe(controlChans)

			for {
				select {
				case <-ctx.Done():
					return
				case <-controlChans.PauseCh:
					t.Log(time.Now(), "Alice paused")
					alicePaused.Store(true)
					// Block until resumed
					controlChans.ResumeCh <- struct{}{}
					aliceResumed.Store(true)
					t.Log(time.Now(), "Alice resumed")
				default:
					// Do work
					time.Sleep(50 * time.Millisecond)
				}
			}
		}()

		// Give Alice time to start
		time.Sleep(200 * time.Millisecond)
		synctest.Wait()

		// Pause
		Pause("Test pause")

		// Wait for controller to be paused
		time.Sleep(200 * time.Millisecond)
		synctest.Wait()

		if !IsPaused() {
			t.Fatal("Controller should be paused")
		}
		if !alicePaused.Load() {
			t.Fatal("Alice should be paused")
		}

		// Now subscribe a second listener AFTER the system is already paused
		wg.Add(1)
		go func() {
			defer wg.Done()
			controlChans := Subscribe()
			defer Unsubscribe(controlChans)

			for {
				select {
				case <-ctx.Done():
					return
				case <-controlChans.PauseCh:
					t.Log(time.Now(), "Bob paused")
					bobPaused.Store(true)
					// Block until resumed
					controlChans.ResumeCh <- struct{}{}
					bobResumed.Store(true)
					t.Log(time.Now(), "Bob resumed")
				default:
					// Do work
					time.Sleep(50 * time.Millisecond)
				}
			}
		}()

		// Give Bob time to subscribe and check pause state
		time.Sleep(200 * time.Millisecond)
		synctest.Wait()

		// Check that Bob is paused
		if !bobPaused.Load() {
			t.Fatal("Bob not paused after subscribing to paused controller")
		}
		t.Log("Bob is paused")

		// Verify both are paused
		if !alicePaused.Load() || !bobPaused.Load() {
			t.Fatal("Both listeners should be paused")
		}
		t.Log("Both listeners are paused")

		// Resume the controller
		Resume()

		// Wait for both listeners to resume
		time.Sleep(500 * time.Millisecond)
		synctest.Wait()

		// Check that both listeners have resumed
		if !aliceResumed.Load() {
			t.Fatal("Alice should have resumed")
		}
		if !bobResumed.Load() {
			t.Fatal("Bob should have resumed")
		}
		if IsPaused() {
			t.Fatal("System should not be paused")
		}
		t.Log("Both listeners resumed ✓")

		// Clean up
		cancel()
		wg.Wait()
	})
}
