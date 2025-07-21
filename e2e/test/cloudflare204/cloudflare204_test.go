package boot

import (
	_ "embed"
	"fmt"
	"os"
	"path"
	"sync"
	"testing"

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

func (rm *recordMatcher) ShouldStop() bool {
	return rm.urlArchived || rm.errored
}

func TestCloudFlare204(t *testing.T) {
	os.RemoveAll("jobs")

	tempSocketPath := path.Join(os.TempDir(), fmt.Sprintf("zeno-%d.sock", os.Getpid()))
	defer os.Remove(tempSocketPath)

	shouldStopCh := make(chan struct{})
	rm := &recordMatcher{}
	wg := &sync.WaitGroup{}

	wg.Add(2)

	go e2e.StartHandleLogRecord(t, wg, rm, tempSocketPath, shouldStopCh)
	go e2e.ExecuteCmdZenoGetURL(t, wg, tempSocketPath, []string{"http://cp.cloudflare.com/"})

	e2e.WaitForGoroutines(t, wg, shouldStopCh)
	rm.Assert(t)
}
