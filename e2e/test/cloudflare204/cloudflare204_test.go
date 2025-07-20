package boot

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/internetarchive/Zeno/e2e"
)

type recordMatcher struct {
	stoppedOK   bool
	urlArchived bool
	errored     bool
}

func (rm *recordMatcher) Match(record map[string]string) {
	if record["msg"] == "done, logs are flushing and will be closed" {
		rm.stoppedOK = true
	}
	if record["msg"] == "url archived" && record["url"] == "http://cp.cloudflare.com/" && record["status"] == "204" {
		rm.urlArchived = true
	}
	if record["level"] == "ERROR" {
		rm.errored = true
	}
}

func (rm *recordMatcher) Assert(t *testing.T) {
	if !rm.stoppedOK {
		t.Error("Zeno did not stop gracefully")
	}
	if !rm.urlArchived {
		t.Error("URL was not archived")
	}
	if rm.errored {
		t.Error("An error was logged during the test")
	}
}

func TestCloudFlare204(t *testing.T) {
	os.RemoveAll("jobs")

	tmpDir := os.TempDir()
	tempSocketPath := path.Join(tmpDir, fmt.Sprintf("zeno-%d.sock", os.Getpid()))
	defer os.Remove(tempSocketPath)

	R, W := io.Pipe()

	rm := &recordMatcher{}
	wg := &sync.WaitGroup{}
	wg.Add(3)

	go e2e.LogRecordProcessorWrapper(t, R, rm, wg)
	go e2e.ExecuteCmdZenoGetURL(t, wg, tempSocketPath, []string{"http://cp.cloudflare.com/"})
	go e2e.ConnectSocketThenCopy(t, wg, W, tempSocketPath)

	time.Sleep(10 * time.Second)

	// send self a termination signal to stop
	syscall.Kill(os.Getpid(), syscall.SIGTERM)

	wg.Wait()
	rm.Assert(t)
}
