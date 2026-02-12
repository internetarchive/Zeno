package deadhosts

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewManager_Disabled(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := NewManager(ctx, false, 3, time.Minute, time.Hour)

	if manager.enabled {
		t.Fatal("Expected manager to be disabled")
	}

	// Should be safe to call methods on disabled manager
	if manager.IsDeadHost("example.com") {
		t.Error("Disabled manager should never return true for IsDeadHost")
	}

	manager.RecordFailure("example.com", errors.New("connection refused"))
	manager.RecordSuccess("example.com")
	manager.Close()
}

func TestNewManager_Enabled(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := NewManager(ctx, true, 3, 10*time.Millisecond, 100*time.Millisecond)
	defer manager.Close()

	if !manager.enabled {
		t.Fatal("Expected manager to be enabled")
	}

	if manager.maxFailures != 3 {
		t.Errorf("Expected maxFailures=3, got %d", manager.maxFailures)
	}
}

func TestIsDeadHost(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := NewManager(ctx, true, 3, time.Minute, time.Hour)
	defer manager.Close()

	// Initially no hosts should be dead
	if manager.IsDeadHost("example.com") {
		t.Error("New host should not be dead")
	}

	// Record failures but not enough to mark as dead
	for i := 0; i < 2; i++ {
		manager.RecordFailure("example.com", errors.New("connection refused"))
	}

	if manager.IsDeadHost("example.com") {
		t.Error("Host should not be dead with only 2 failures")
	}

	// Third failure should mark as dead
	manager.RecordFailure("example.com", errors.New("connection refused"))

	if !manager.IsDeadHost("example.com") {
		t.Error("Host should be dead after 3 failures")
	}
}

func TestRecordSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := NewManager(ctx, true, 3, time.Minute, time.Hour)
	defer manager.Close()

	// Record enough failures to mark as dead
	for i := 0; i < 3; i++ {
		manager.RecordFailure("example.com", errors.New("connection refused"))
	}

	if !manager.IsDeadHost("example.com") {
		t.Fatal("Host should be dead after 3 failures")
	}

	// Success should remove from dead hosts
	manager.RecordSuccess("example.com")

	if manager.IsDeadHost("example.com") {
		t.Error("Host should not be dead after success")
	}
}

func TestIsDeadHostError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		expectDead bool
	}{
		{"nil error", nil, false},
		{"connection refused", errors.New("connection refused"), true},
		{"no such host", errors.New("no such host"), true},
		{"network unreachable", errors.New("network is unreachable"), true},
		{"host unreachable", errors.New("host is unreachable"), true},
		{"no route to host", errors.New("no route to host"), true},
		{"dns lookup failed", errors.New("dns lookup failed"), true},
		{"server misbehaving", errors.New("server misbehaving"), true},
		{"timeout error", &timeoutError{}, true},
		{"other error", errors.New("some other error"), false},
		{"http 500", errors.New("500 Internal Server Error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDeadHostError(tt.err)
			if result != tt.expectDead {
				t.Errorf("isDeadHostError(%v) = %v; want %v", tt.err, result, tt.expectDead)
			}
		})
	}
}

func TestRecordFailure_NonDeadHostError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := NewManager(ctx, true, 3, time.Minute, time.Hour)
	defer manager.Close()

	// Record non-dead-host errors
	for i := 0; i < 5; i++ {
		manager.RecordFailure("example.com", errors.New("500 Internal Server Error"))
	}

	// Should not be marked as dead
	if manager.IsDeadHost("example.com") {
		t.Error("Host should not be dead for non-dead-host errors")
	}
}

func TestCleanup(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := NewManager(ctx, true, 3, 10*time.Millisecond, 50*time.Millisecond)
	defer manager.Close()

	// Record failures to mark as dead
	for i := 0; i < 3; i++ {
		manager.RecordFailure("example.com", errors.New("connection refused"))
	}

	if !manager.IsDeadHost("example.com") {
		t.Fatal("Host should be dead")
	}

	// Wait for cleanup to occur
	time.Sleep(100 * time.Millisecond)

	// Host should be cleaned up due to age
	if manager.IsDeadHost("example.com") {
		t.Error("Host should have been cleaned up")
	}
}

func TestGetStats(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := NewManager(ctx, true, 3, time.Minute, time.Hour)
	defer manager.Close()

	// Initially no stats
	dead, total := manager.GetStats()
	if dead != 0 || total != 0 {
		t.Errorf("Expected (0,0), got (%d,%d)", dead, total)
	}

	// Record some failures but not enough to mark as dead
	manager.RecordFailure("example1.com", errors.New("connection refused"))
	manager.RecordFailure("example1.com", errors.New("connection refused"))

	dead, total = manager.GetStats()
	if dead != 0 || total != 1 {
		t.Errorf("Expected (0,1), got (%d,%d)", dead, total)
	}

	// Mark one as dead
	manager.RecordFailure("example1.com", errors.New("connection refused"))

	// Add another host with failures
	manager.RecordFailure("example2.com", errors.New("no such host"))

	dead, total = manager.GetStats()
	if dead != 1 || total != 2 {
		t.Errorf("Expected (1,2), got (%d,%d)", dead, total)
	}
}

func TestGetStats_Disabled(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := NewManager(ctx, false, 3, time.Minute, time.Hour)

	dead, total := manager.GetStats()
	if dead != 0 || total != 0 {
		t.Errorf("Disabled manager should return (0,0), got (%d,%d)", dead, total)
	}
}

// timeoutError implements net.Error for testing timeout scenarios
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return false }

func TestClose(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := NewManager(ctx, true, 3, 10*time.Millisecond, time.Hour)

	// Close should not hang
	done := make(chan struct{})
	go func() {
		manager.Close()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("Close() took too long")
	}
}
