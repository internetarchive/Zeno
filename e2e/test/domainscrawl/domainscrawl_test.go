package domainscrawl

import (
	_ "embed"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/internetarchive/Zeno/e2e"
)

type recordMatcher struct {
	stoppedOK             bool
	mainPageProcessed     bool
	level1PageProcessed   bool
	level1_2PageProcessed bool
	level2PageSkipped     bool
	level2_2PageProcessed bool
	level3_2PageProcessed bool
	hopCountSetToZero     bool
	errored               bool
	hopCountResetURLs     []string
}

func (rm *recordMatcher) Match(record map[string]string) {
	if record["msg"] == "done, logs are flushing and will be closed" {
		rm.stoppedOK = true
	}

	// Check for main page processing
	if record["msg"] == "processed body" && strings.Contains(record["url"], "/") && record["status_code"] == "200" {
		rm.mainPageProcessed = true
	}

	// Check for level 1 page processing
	if record["msg"] == "processed body" && strings.Contains(record["url"], "/level1") && record["status_code"] == "200" {
		rm.level1PageProcessed = true
	}

	// Check for level 1-2 page processing
	if record["msg"] == "processed body" && strings.Contains(record["url"], "/level1-2") && record["status_code"] == "200" {
		rm.level1_2PageProcessed = true
	}

	// Check for level 2 page being skipped
	if record["msg"] == "skipping outlink due to hop count" && strings.Contains(record["url"], "/level2") {
		rm.level2PageSkipped = true
	}

	// Check for level 2-2 page processing (should happen due to domains-crawl)
	if record["msg"] == "processed body" && strings.Contains(record["url"], "/level2-2") && record["status_code"] == "200" {
		rm.level2_2PageProcessed = true
	}

	// Check for level 3-2 page processing (should happen due to domains-crawl)
	if record["msg"] == "processed body" && strings.Contains(record["url"], "/level3-2") && record["status_code"] == "200" {
		rm.level3_2PageProcessed = true
	}

	// Check for hop count being set to 0 due to domains-crawl
	if record["msg"] == "setting hop count to 0 (domains crawl)" {
		rm.hopCountSetToZero = true
		rm.hopCountResetURLs = append(rm.hopCountResetURLs, record["url"])
	}

	if record["level"] == "ERROR" {
		rm.errored = true
	}
}

func (rm *recordMatcher) Assert(t *testing.T) {
	if !rm.stoppedOK {
		t.Error("Zeno did not stop gracefully")
	}
	if !rm.hopCountSetToZero {
		t.Error("No URLs had their hop count set to 0 due to domains-crawl")
	}
	if len(rm.hopCountResetURLs) < 4 {
		t.Errorf("Expected at least 4 URLs to have hop count reset, got %d", len(rm.hopCountResetURLs))
	}
	if rm.errored {
		t.Error("An error was logged during the test")
	}

	// Verify that the URLs that had hop count reset are from our test domain
	for _, url := range rm.hopCountResetURLs {
		if !strings.Contains(url, "127.0.0.1.nip.io") {
			t.Errorf("URL %s had hop count reset but doesn't match domains-crawl pattern", url)
		}
	}

	// Log which URLs had their hop count reset for debugging
	if len(rm.hopCountResetURLs) > 0 {
		t.Logf("URLs that had hop count reset: %v", rm.hopCountResetURLs)
	}
}

func (rm *recordMatcher) ShouldStop() bool {
	// Stop when we've seen the key domains-crawl behavior or if there's an error
	// We need at least 4 hop count resets to ensure the feature is working
	return (rm.hopCountSetToZero && len(rm.hopCountResetURLs) >= 4) || rm.errored
}

func TestDomainsCrawl(t *testing.T) {
	server := SetupServer()
	serverURL := strings.Replace(server.URL, "127.0.0.1", "127.0.0.1.nip.io", 1)
	defer server.Close()
	os.RemoveAll("jobs")
	defer os.RemoveAll("jobs")

	shouldStopCh := make(chan struct{})
	rm := &recordMatcher{}
	wg := &sync.WaitGroup{}

	wg.Add(2)

	go e2e.StartHandleLogRecord(t, wg, rm, shouldStopCh)
	go e2e.ExecuteCmdZenoGetURL(t, wg, []string{serverURL + "/"})

	e2e.WaitForGoroutines(t, wg, shouldStopCh)
	rm.Assert(t)
}
