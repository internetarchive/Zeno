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
	expectedURLs    []string
	mt              sync.Mutex
}

func newRecordMatcher() *recordMatcher {
	return &recordMatcher{
		archivedURLs: make(map[string]bool),
	}
}

func (rm *recordMatcher) Match(record map[string]string) {
	if record["level"] == "INFO" {
		if strings.Contains(record["msg"], "url archived") {
			if url, ok := record["url"]; ok {
				rm.mt.Lock()
				rm.archivedURLs[url] = true
				rm.mt.Unlock()
			}
		}
	}
	if record["level"] == "ERROR" {
		rm.mt.Lock()
		rm.unexpectedError = true
		rm.mt.Unlock()
	}
}

func (rm *recordMatcher) Assert(t *testing.T) {
	rm.mt.Lock()
	defer rm.mt.Unlock()

	for _, expectedURL := range rm.expectedURLs {
		if !rm.archivedURLs[expectedURL] {
			t.Errorf("Zeno did not archive expected URL: %s", expectedURL)
		}
	}

	if rm.unexpectedError {
		t.Error("An unexpected error was logged during the test")
	}
}

func (rm *recordMatcher) ShouldStop() bool {
	rm.mt.Lock()
	defer rm.mt.Unlock()

	if rm.unexpectedError {
		return true
	}

	for _, urls := range rm.expectedURLs {
		if !rm.archivedURLs[urls] {
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
