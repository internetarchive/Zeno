package e2e

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/internetarchive/Zeno/cmd"
	"github.com/internetarchive/Zeno/e2e/log"
	"github.com/spf13/cobra"
)

func CmdZenoGetURL(socketPath string, urls []string) *cobra.Command {
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

func ConnectSocketThenCopy(t *testing.T, wg *sync.WaitGroup, W *io.PipeWriter, socketPath string) {
	defer wg.Done()
	conn, err := lazyDial(socketPath, 5*time.Second)
	if err != nil {
		t.Errorf("failed to connect to log socket: %v", err)
	}
	defer conn.Close()
	io.Copy(W, conn)
	defer W.Close()
}

func ExecuteCmdZenoGetURL(t *testing.T, wg *sync.WaitGroup, socketPath string, urls []string) {
	defer wg.Done()
	cmdErr := CmdZenoGetURL(socketPath, urls).Execute()
	if cmdErr != nil {
		t.Errorf("failed to start command: %v", cmdErr)
	}
}

func LogRecordProcessorWrapper(t *testing.T, R *io.PipeReader, rm log.RecordMatcher, wg *sync.WaitGroup) {
	defer wg.Done()
	err := log.LogRecordProcessor(R, rm.Match)
	if err != nil {
		t.Error("failed to listen to logs:", err)
	}
}
