package getlist

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/internetarchive/Zeno/v2/e2e"
)

type recordMatcher struct {
	serverURL       string
	archivedURLs    map[string]bool
	unexpectedError bool
}

func newRecordMatcher(serverURL string) *recordMatcher {
	return &recordMatcher{
		serverURL:    serverURL,
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
		rm.serverURL + "/",
		rm.serverURL + "/page1",
		rm.serverURL + "/page2",
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
	server := setupServer()
	serverURL := strings.Replace(server.URL, "127.0.0.1", "127.0.0.1.nip.io", 1)
	defer server.Close()
	os.RemoveAll("jobs")
	defer os.RemoveAll("jobs")

	// The root URL is listed without a trailing slash on purpose, Zeno should normalize it
	urlsFile := filepath.Join(t.TempDir(), "urls.txt")
	urls := "# Test URL list for e2e test\n" + serverURL + "\n" + serverURL + "/page1\n" + serverURL + "/page2\n"
	if err := os.WriteFile(urlsFile, []byte(urls), 0644); err != nil {
		t.Fatalf("unable to write URL list: %v", err)
	}

	shouldStopCh := make(chan struct{})
	rm := newRecordMatcher(serverURL)
	wg := &sync.WaitGroup{}

	wg.Add(2)

	go e2e.StartHandleLogRecord(t, wg, rm, shouldStopCh)
	go e2e.ExecuteCmdZenoGetList(t, wg, []string{urlsFile})

	e2e.WaitForGoroutines(t, wg, shouldStopCh)
	rm.Assert(t)
}

func setupServer() *httptest.Server {
	mux := http.NewServeMux()

	for _, path := range []string{"/", "/page1", "/page2"} {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != path {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html><head><title>getlist</title></head><body><p>" + path + "</p></body></html>"))
		})
	}

	return httptest.NewServer(mux)
}
