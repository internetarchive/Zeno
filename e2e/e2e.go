package e2e

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/internetarchive/Zeno/cmd"
	"github.com/internetarchive/Zeno/e2e/log"
	"github.com/spf13/cobra"
)

var DefaultTimeout = 60 * time.Second

func cmdZenoGetURL(socketPath string, urls []string) *cobra.Command {
	cmd := cmd.Prepare()
	args := append([]string{"get", "url", "--config-file", "config.toml", "--log-socket-level", "debug", "--log-socket", socketPath}, urls...)
	fmt.Println("Command arguments:", args)
	cmd.SetArgs(args)
	return cmd
}

func lazyDial(socketPath string, timeout time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var conn net.Conn
	var err error

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout while waiting for socket %s", socketPath)
		default:
			conn, err = net.Dial("unix", socketPath)
			if err == nil {
				return conn, nil
			}
			time.Sleep(100 * time.Millisecond) // Retry
		}
	}
}

// connects to [socketPath] and copies logs from it to [W] until the connection is closed
func connectSocketThenCopy(t *testing.T, wg *sync.WaitGroup, W *io.PipeWriter, socketPath string) {
	defer wg.Done()
	conn, err := lazyDial(socketPath, 5*time.Second)
	if err != nil {
		t.Errorf("failed to connect to log socket: %v", err)
	}
	defer conn.Close()
	io.Copy(W, conn)
	defer W.Close()
}

// ExecuteCmdZenoGetURL executes the Zeno get URL command with the provided socket path, URLs and custom config.toml file
func ExecuteCmdZenoGetURL(t *testing.T, wg *sync.WaitGroup, socketPath string, urls []string) {
	defer wg.Done()
	cmdErr := cmdZenoGetURL(socketPath, urls).Execute()
	if cmdErr != nil {
		t.Errorf("failed to start command: %v", cmdErr)
	}
}

// Connects to the log socket and processes log records using the provided RecordMatcher
func StartHandleLogRecord(t *testing.T, wg *sync.WaitGroup, rm log.RecordMatcher, socketPath string, stopCh chan struct{}) {
	defer wg.Done()
	R, W := io.Pipe()
	wg.Add(1)
	go connectSocketThenCopy(t, wg, W, socketPath)

	err := log.LogRecordProcessor(R, rm, stopCh)
	if err != nil {
		t.Error("failed to listen to logs:", err)
	}

}

// WaitForGoroutines waits for ANY of the following conditions to be met:
//
//   - [1] all goroutines in the wait group to finish
//   - [2] the test deadline is reached
//   - [3] the shouldStop channel is closed
//
// If [2], [3] are met, it will send a termination signal to the current process to gracefully stop Zeno.
// If [2] is met, mark the test as failed.
func WaitForGoroutines(t *testing.T, wg *sync.WaitGroup, shouldStopCh chan struct{}) {
	deadline, _ := t.Deadline()
	// wait until the deadline is reached or the test is done
	wgDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
		t.Log("All goroutines finished")
	case <-time.After(time.Until(deadline)):
		t.Error("Test timed out before all goroutines finished")
		// send self a termination signal to stop
		syscall.Kill(os.Getpid(), syscall.SIGTERM)
	case <-shouldStopCh:
		t.Log("Should stop channel received a signal, stopping test")
		// send self a termination signal to stop
		syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}

	wg.Wait()
}
