//go:build !windows

package lq

import (
	"os"
	"syscall"
)

// sendTerminationSignal sends a SIGTERM signal to the current process to trigger graceful shutdown.
// This works on Unix-like systems (Linux, macOS, etc.).
func sendTerminationSignal() error {
	pid := os.Getpid()
	return syscall.Kill(pid, syscall.SIGTERM)
}
