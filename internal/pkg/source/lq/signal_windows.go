//go:build windows

package lq

import (
	"os"
)

// sendTerminationSignal triggers a clean exit on Windows.
// Windows doesn't have the same signal mechanism, so we use os.Exit(0).
func sendTerminationSignal() error {
	// On Windows, use os.Exit for clean termination
	os.Exit(0)
	return nil // This line will never be reached
}
