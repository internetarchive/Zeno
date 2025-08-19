package e2e

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/internetarchive/Zeno/cmd"
	"github.com/internetarchive/Zeno/e2e/log"
	"github.com/spf13/cobra"

	"github.com/internetarchive/Zeno/internal/pkg/controler"
	zenolog "github.com/internetarchive/Zeno/internal/pkg/log"
)

var DefaultTimeout = 60 * time.Second
var DialTimeout = 10 * time.Second

func cmdZenoGetURL(urls []string) *cobra.Command {
	cmd := cmd.Prepare()
	args := append([]string{"get", "url", "--config-file", "config.toml", "--log-e2e", "--log-e2e-level", "debug", "--no-stdout-log", "--no-stderr-log"}, urls...)
	fmt.Println("Command arguments:", args)
	cmd.SetArgs(args)
	return cmd
}

func lazyDial(timeout time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout while waiting for connection")
		default:
			if zenolog.E2EConnCfg != nil && zenolog.E2EConnCfg.ConnR != nil {
				return zenolog.E2EConnCfg.ConnR, nil
			}
			time.Sleep(100 * time.Millisecond) // Retry
		}
	}
}

// connects to the log conn and copies logs from it to [W] until the connection is closed
func connectThenCopy(t *testing.T, wg *sync.WaitGroup, W *io.PipeWriter) {
	defer wg.Done()
	conn, err := lazyDial(DialTimeout)
	if err != nil {
		t.Errorf("failed to connect to log conn: %v", err)
	}
	defer conn.Close()
	io.Copy(W, conn)
	defer W.Close()
}

// ExecuteCmdZenoGetURL executes the Zeno get URL command with the e2e logging, URLs and custom config.toml file
func ExecuteCmdZenoGetURL(t *testing.T, wg *sync.WaitGroup, urls []string) {
	defer wg.Done()
	cmdErr := cmdZenoGetURL(urls).Execute()
	if cmdErr != nil {
		t.Errorf("failed to start command: %v", cmdErr)
	}
}

// Connects to the log conn and processes log records using the provided RecordMatcher
func StartHandleLogRecord(t *testing.T, wg *sync.WaitGroup, rm log.RecordMatcher, stopCh chan struct{}) {
	defer wg.Done()
	R, W := io.Pipe()
	wg.Add(1)
	go connectThenCopy(t, wg, W)

	err := log.LogRecordProcessor(R, rm, stopCh)
	if err != nil {
		t.Error("failed to listen to logs:", err)
	}

}

// WaitForGoroutines waits for ANY of the following conditions to be met:
//
//   - [1] all goroutines in the wait group to finish
//   - [2] the shouldStop channel is closed
//
// Then send a termination signal to the Zeno controler to gracefully stop Zeno.
func WaitForGoroutines(t *testing.T, wg *sync.WaitGroup, shouldStopCh chan struct{}) {
	wgDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
		t.Log("All goroutines finished")
	case <-shouldStopCh:
		t.Log("Should stop channel received a signal, stopping test")
		controler.SignalChan <- os.Interrupt
	}

	wg.Wait()
}
