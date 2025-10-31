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
	urlArchived     bool
	unexpectedError bool
}

func (rm *recordMatcher) Match(record map[string]string) {
	if record["level"] == "INFO" {
		if strings.Contains(record["msg"], "url archived") {
			rm.urlArchived = true
		}
	}
	if record["level"] == "ERROR" {
		rm.unexpectedError = true
	}
}

func (rm *recordMatcher) Assert(t *testing.T) {
	if !rm.urlArchived {
		t.Error("Zeno did not archive the URL from the list")
	}
	if rm.unexpectedError {
		t.Error("An unexpected error was logged during the test")
	}
}

func (rm *recordMatcher) ShouldStop() bool {
	return rm.urlArchived || rm.unexpectedError
}

func TestGetList(t *testing.T) {
	os.RemoveAll("jobs")
	defer os.RemoveAll("jobs")

	shouldStopCh := make(chan struct{})
	rm := &recordMatcher{}
	wg := &sync.WaitGroup{}

	wg.Add(2)

	go e2e.StartHandleLogRecord(t, wg, rm, shouldStopCh)
	go e2e.ExecuteCmdZenoGetList(t, wg, []string{"urls.txt"})

	e2e.WaitForGoroutines(t, wg, shouldStopCh)
	rm.Assert(t)
}
