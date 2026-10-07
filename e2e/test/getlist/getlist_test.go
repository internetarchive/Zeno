package getlist

import (
	_ "embed"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/internetarchive/Zeno/v2/e2e"
)

type recordMatcher struct {
	archivedURLs    map[string]bool
	unexpectedError bool
}

func newRecordMatcher() *recordMatcher {
	return &recordMatcher{
		archivedURLs: make(map[string]bool),
	}
}

func (rm *recordMatcher) Match(record map[string]string) {
	if record["level"] == "INFO" {
		if strings.Contains(record["msg"], "url archived") {
			// Extract URL from the log record
			if url, ok := record["url"]; ok {
				rm.archivedURLs[url] = true
			}
		}
	}
	if record["level"] == "ERROR" {
		rm.unexpectedError = true
	}
}

var expectedURLs = []string{
	"https://example.com/",
	"https://example.com/page1",
	"https://example.com/page2",
}

func (rm *recordMatcher) Assert(t *testing.T) {
	for _, expectedURL := range expectedURLs {
		if !rm.archivedURLs[expectedURL] {
			t.Errorf("Zeno did not archive expected URL: %s", expectedURL)
		}
	}

	if rm.unexpectedError {
		t.Error("An unexpected error was logged during the test")
	}
}

func (rm *recordMatcher) ShouldStop() bool {
	// Stop when we've archived all expected URLs or encountered an error.
	// Assets (e.g. example.com/s.js) are archived too, so counting archived
	// URLs is not enough.
	if rm.unexpectedError {
		return true
	}
	for _, expectedURL := range expectedURLs {
		if !rm.archivedURLs[expectedURL] {
			return false
		}
	}
	return true
}

func TestGetList(t *testing.T) {
	os.RemoveAll("jobs")
	defer os.RemoveAll("jobs")

	shouldStopCh := make(chan struct{})
	rm := newRecordMatcher()
	wg := &sync.WaitGroup{}

	wg.Add(2)

	go e2e.StartHandleLogRecord(t, wg, rm, shouldStopCh)
	go e2e.ExecuteCmdZenoGetList(t, wg, []string{"urls.txt"})

	e2e.WaitForGoroutines(t, wg, shouldStopCh)
	rm.Assert(t)
}
