package deadhosts

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Integration test that demonstrates the dead hosts feature working with realistic scenarios
func TestDeadHostsIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Test with dead hosts enabled
	manager := NewManager(ctx, true, 3, 100*time.Millisecond, 500*time.Millisecond)
	defer manager.Close()

	host := "deadhost.example.com"

	// Initially, host should not be dead
	if manager.IsDeadHost(host) {
		t.Fatal("Host should not be dead initially")
	}

	// Record connection refused errors (common dead host scenario)
	for i := 0; i < 2; i++ {
		manager.RecordFailure(host, errors.New("dial tcp: connect: connection refused"))
	}

	// Should not be dead yet
	if manager.IsDeadHost(host) {
		t.Error("Host should not be dead with only 2 failures")
	}

	// One more failure should mark as dead
	manager.RecordFailure(host, errors.New("dial tcp: connect: connection refused"))

	if !manager.IsDeadHost(host) {
		t.Error("Host should be dead after 3 connection refused errors")
	}

	// Test that a successful connection removes from dead hosts
	manager.RecordSuccess(host)

	if manager.IsDeadHost(host) {
		t.Error("Host should not be dead after successful connection")
	}

	// Test DNS failure scenario
	dnsHost := "dnshost.example.com"
	manager.RecordFailure(dnsHost, errors.New("lookup dnshost.example.com: no such host"))
	manager.RecordFailure(dnsHost, errors.New("lookup dnshost.example.com: no such host"))
	manager.RecordFailure(dnsHost, errors.New("lookup dnshost.example.com: no such host"))

	if !manager.IsDeadHost(dnsHost) {
		t.Error("Host should be dead after 3 DNS failures")
	}

	// Test timeout scenario
	timeoutHost := "timeouthost.example.com"
	timeoutErr := &testTimeoutError{}
	manager.RecordFailure(timeoutHost, timeoutErr)
	manager.RecordFailure(timeoutHost, timeoutErr)
	manager.RecordFailure(timeoutHost, timeoutErr)

	if !manager.IsDeadHost(timeoutHost) {
		t.Error("Host should be dead after 3 timeout failures")
	}

	// Verify stats
	deadCount, totalTracked := manager.GetStats()
	expectedDead := 2  // dnsHost and timeoutHost should be dead
	expectedTotal := 2 // host was removed by success, dnsHost and timeoutHost remain

	if deadCount != expectedDead || totalTracked != expectedTotal {
		t.Errorf("Expected stats (%d,%d), got (%d,%d)", expectedDead, expectedTotal, deadCount, totalTracked)
	}

	// Test that non-dead-host errors don't mark as dead
	httpHost := "httphost.example.com"
	for i := 0; i < 5; i++ {
		manager.RecordFailure(httpHost, errors.New("HTTP 500 Internal Server Error"))
	}

	if manager.IsDeadHost(httpHost) {
		t.Error("Host should not be dead from HTTP errors")
	}

	// Wait for cleanup to occur
	time.Sleep(600 * time.Millisecond)

	// Hosts should be cleaned up due to age
	deadCount, totalTracked = manager.GetStats()
	if deadCount > 0 {
		t.Errorf("Expected all dead hosts to be cleaned up, but %d still marked as dead", deadCount)
	}
}

// Test disabled manager doesn't interfere
func TestDeadHostsIntegration_Disabled(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager := NewManager(ctx, false, 3, time.Minute, time.Hour)

	host := "deadhost.example.com"

	// Should always return false when disabled
	for i := 0; i < 10; i++ {
		manager.RecordFailure(host, errors.New("connection refused"))
		if manager.IsDeadHost(host) {
			t.Fatal("Disabled manager should never mark hosts as dead")
		}
	}

	// Stats should always be 0 when disabled
	deadCount, totalTracked := manager.GetStats()
	if deadCount != 0 || totalTracked != 0 {
		t.Errorf("Disabled manager should return (0,0) stats, got (%d,%d)", deadCount, totalTracked)
	}

	// Close should be safe on disabled manager
	manager.Close()
}

// testTimeoutError implements net.Error for testing timeout scenarios
type testTimeoutError struct{}

func (e *testTimeoutError) Error() string   { return "timeout" }
func (e *testTimeoutError) Timeout() bool   { return true }
func (e *testTimeoutError) Temporary() bool { return false }
