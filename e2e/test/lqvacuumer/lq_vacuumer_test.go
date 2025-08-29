package nxdomain

import (
	_ "embed"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/internetarchive/Zeno/e2e"
	"github.com/internetarchive/Zeno/internal/pkg/source/lq"
)

type recordMatcher struct {
	vacuumedSuccess bool
	unexpectedError bool
}

func (rm *recordMatcher) Match(record map[string]string) {
	if record["level"] == "INFO" && record["msg"] == "vacuuming complete" {
		rm.vacuumedSuccess = true
	}
	if record["level"] == "ERROR" {
		if strings.Contains(record["err"], "failed to resolve DNS") {
		} else {
			rm.unexpectedError = true
		}
	}
}

func (rm *recordMatcher) Assert(t *testing.T) {
	if rm.unexpectedError {
		t.Error("An unexpected error was logged during the test")
	}
}

func (rm *recordMatcher) ShouldStop() bool {
	return rm.vacuumedSuccess || rm.unexpectedError
}

func Test_LQ_vacuumer(t *testing.T) {
	os.RemoveAll("jobs")

	rm := &recordMatcher{}
	wg := &sync.WaitGroup{}
	shouldStopCh := make(chan struct{})
	wg.Add(2)

	lq.VacuumInterval = 1 * time.Second

	go e2e.StartHandleLogRecord(t, wg, rm, shouldStopCh)
	go e2e.ExecuteCmdZenoGetURL(t, wg, []string{"http://nxdomain.nxtld/"})

	e2e.WaitForGoroutines(t, wg, shouldStopCh)
	rm.Assert(t)

}
