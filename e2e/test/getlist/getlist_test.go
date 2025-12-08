package getlist

import (
	_ "embed"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/internetarchive/Zeno/e2e"
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

func (rm *recordMatcher) Assert(t *testing.T) {
	expectedURLs := []string{
		"https://example.com/",
		"https://example.com/page1",
		"https://example.com/page2",
	}

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
	// Stop when we've archived all 3 expected URLs or encountered an error
	return len(rm.archivedURLs) >= 3 || rm.unexpectedError
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
