package nxdomain

import (
	_ "embed"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/internetarchive/Zeno/e2e"
)

type recordMatcher struct {
	failedToResolve bool
	unexpectedError bool
}

func (rm *recordMatcher) Match(record map[string]string) {
	if record["level"] == "ERROR" {
		if strings.Contains(record["err"], "no suitable IP address found") {
			rm.failedToResolve = true
		} else {
			rm.unexpectedError = true
		}
	}
}

func (rm *recordMatcher) Assert(t *testing.T) {
	if !rm.failedToResolve {
		t.Error("Zeno did not fail to resolve the NXDOMAIN URL")
	}
	if rm.unexpectedError {
		t.Error("An unexpected error was logged during the test")
	}
}

func (rm *recordMatcher) ShouldStop() bool {
	return rm.failedToResolve || rm.unexpectedError
}

func TestNXDomain(t *testing.T) {
	os.RemoveAll("jobs")
	defer os.RemoveAll("jobs")

	shouldStopCh := make(chan struct{})
	rm := &recordMatcher{}
	wg := &sync.WaitGroup{}

	wg.Add(2)

	go e2e.StartHandleLogRecord(t, wg, rm, shouldStopCh)
	go e2e.ExecuteCmdZenoGetURL(t, wg, []string{"http://nxdomain.nxtld/"})

	e2e.WaitForGoroutines(t, wg, shouldStopCh)
	rm.Assert(t)
}
