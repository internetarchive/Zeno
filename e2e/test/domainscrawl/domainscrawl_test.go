package domainscrawl

import (
	"net/http"
	"net/http/httptest"
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
	server := setupServer()
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

func setupServer() *httptest.Server {
	mux := http.NewServeMux()

	// Main page with links to other pages
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := `<html>
<head><title>Test Page</title></head>
<body>
<h1>Test Page</h1>
<p>This is the main test page.</p>
<a href="/level1">Level 1</a>
<a href="/external">External Link</a>
</body>
</html>`
		w.Write([]byte(html))
	})

	// Level 1 page (hop 1)
	mux.HandleFunc("/level1", func(w http.ResponseWriter, r *http.Request) {
		html := `<html>
<head><title>Level 1</title></head>
<body>
<h1>Level 1</h1>
<p>This is level 1.</p>
<a href="/level2">Level 2</a>
<a href="/level1-2">Level 1-2</a>
</body>
</html>`
		w.Write([]byte(html))
	})

	// Level 2 page (hop 2) - should be skipped without domains-crawl
	mux.HandleFunc("/level2", func(w http.ResponseWriter, r *http.Request) {
		html := `<html>
<head><title>Level 2</title></head>
<body>
<h1>Level 2</h1>
<p>This is level 2.</p>
<a href="/level3">Level 3</a>
</body>
</html>`
		w.Write([]byte(html))
	})

	// Another level 1 page
	mux.HandleFunc("/level1-2", func(w http.ResponseWriter, r *http.Request) {
		html := `<html>
<head><title>Level 1-2</title></head>
<body>
<h1>Level 1-2</h1>
<p>This is another level 1 page.</p>
<a href="/level2-2">Level 2-2</a>
</body>
</html>`
		w.Write([]byte(html))
	})

	// Level 2-2 page (hop 2) - should be crawled with domains-crawl
	mux.HandleFunc("/level2-2", func(w http.ResponseWriter, r *http.Request) {
		html := `<html>
<head><title>Level 2-2</title></head>
<body>
<h1>Level 2-2</h1>
<p>This is level 2-2.</p>
<a href="/level3-2">Level 3-2</a>
</body>
</html>`
		w.Write([]byte(html))
	})

	// Level 3-2 page (hop 3) - should be crawled with domains-crawl
	mux.HandleFunc("/level3-2", func(w http.ResponseWriter, r *http.Request) {
		html := `<html>
<head><title>Level 3-2</title></head>
<body>
<h1>Level 3-2</h1>
<p>This is level 3-2.</p>
</body>
</html>`
		w.Write([]byte(html))
	})

	// External link (should not be crawled)
	mux.HandleFunc("/external", func(w http.ResponseWriter, r *http.Request) {
		html := `<html>
<head><title>External</title></head>
<body>
<h1>External</h1>
<p>This is an external link.</p>
</body>
</html>`
		w.Write([]byte(html))
	})

	return httptest.NewServer(mux)
}
