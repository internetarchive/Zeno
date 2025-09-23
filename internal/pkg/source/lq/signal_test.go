//go:build testing

package lq

// sendTerminationSignal is a no-op during testing to avoid os.Exit() calls.
// This allows e2e tests to complete without the process terminating.
func sendTerminationSignal() error {
	// No-op during testing
	return nil
}
