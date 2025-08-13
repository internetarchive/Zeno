package example_com

import (
	_ "embed"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/internetarchive/Zeno/e2e"
)

type recordMatcher struct {
	stoppedOK        bool
	urlBodyProcessed bool
	pageArchived     bool
	errored          bool
}

func (rm *recordMatcher) Match(record map[string]string) {
	if record["msg"] == "done, logs are flushing and will be closed" {
		rm.stoppedOK = true
	}
	if record["msg"] == "processed body" && strings.Contains(record["url"], "/ok") && record["status_code"] == "200" {
		rm.urlBodyProcessed = true
	}
	if record["msg"] == "page archived successfully" && strings.Contains(record["item_url"], "/ok") {
		rm.pageArchived = true
	}
	if record["level"] == "ERROR" {
		rm.errored = true
	}
}

func (rm *recordMatcher) Assert(t *testing.T) {
	if !rm.stoppedOK {
		t.Error("Zeno did not stop gracefully")
	}
	if !rm.urlBodyProcessed {
		t.Error("URL body was not processed")
	}
	if !rm.pageArchived {
		t.Error("Page was not archived")
	}
	if rm.errored {
		t.Error("An error was logged during the test")
	}
}

func (rm *recordMatcher) ShouldStop() bool {
	return (rm.urlBodyProcessed && rm.pageArchived) || rm.errored
}

func TestStatusOK(t *testing.T) {
	server := SetupServer()
	serverURL := strings.Replace(server.URL, "127.0.0.1", "127.0.0.1.nip.io", 1)
	defer server.Close()
	os.RemoveAll("jobs")

	tempSocketPath := path.Join(os.TempDir(), fmt.Sprintf("zeno-%d.sock", os.Getpid()))
	defer os.Remove(tempSocketPath)

	shouldStopCh := make(chan struct{})
	rm := &recordMatcher{}
	wg := &sync.WaitGroup{}

	wg.Add(2)

	go e2e.StartHandleLogRecord(t, wg, rm, tempSocketPath, shouldStopCh)
	go e2e.ExecuteCmdZenoGetURL(t, wg, tempSocketPath, []string{serverURL + "/ok"})

	e2e.WaitForGoroutines(t, wg, shouldStopCh)
	rm.Assert(t)
}
