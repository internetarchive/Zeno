package autofinish

import (
	_ "embed"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/internetarchive/Zeno/e2e"
)

type recordMatcher struct {
	crawlFinished    bool
	stoppedOK        bool
	unexpectedError  bool
	urlProcessed     bool
}

func (rm *recordMatcher) Match(record map[string]string) {
	if record["msg"] == "crawl finished: no URLs in queue and no active work in reactor, triggering graceful shutdown" {
		rm.crawlFinished = true
	}
	if record["msg"] == "done, logs are flushing and will be closed" {
		rm.stoppedOK = true
	}
	if record["level"] == "ERROR" && !strings.Contains(record["err"], "unsupported host") {
		// We expect "unsupported host" errors for localhost URLs in the test environment
		rm.unexpectedError = true
	}
	if record["msg"] == "seed finished" {
		rm.urlProcessed = true
	}
}

func (rm *recordMatcher) Assert(t *testing.T) {
	if !rm.crawlFinished {
		t.Error("Zeno did not detect crawl finished automatically")
	}
	if !rm.stoppedOK {
		t.Error("Zeno did not stop gracefully")
	}
	if !rm.urlProcessed {
		t.Error("URL was not processed through the pipeline")
	}
	if rm.unexpectedError {
		t.Error("An unexpected error was logged during the test")
	}
}

func (rm *recordMatcher) ShouldStop() bool {
	return rm.stoppedOK || rm.unexpectedError
}

func TestAutoFinish(t *testing.T) {
	os.RemoveAll("jobs")
	defer os.RemoveAll("jobs")

	shouldStopCh := make(chan struct{})
	rm := &recordMatcher{}
	wg := &sync.WaitGroup{}

	wg.Add(2)

	go e2e.StartHandleLogRecord(t, wg, rm, shouldStopCh)
	// Use localhost URL that will fail validation quickly but still go through the pipeline
	go e2e.ExecuteCmdZenoGetURL(t, wg, []string{"http://localhost:9999/autofinish-test"})

	e2e.WaitForGoroutines(t, wg, shouldStopCh)
	rm.Assert(t)
}