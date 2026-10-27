package sitemapindex

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/internetarchive/Zeno/v2/e2e"
)

type recordMatcher struct {
	mu            sync.Mutex
	crawlFinished bool
	errored       bool
	archivedHops  map[string]string // path -> hops
}

func (rm *recordMatcher) Match(record map[string]string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if record["msg"] == "crawl finished: no URLs in queue and no active work in reactor, triggering graceful shutdown" {
		rm.crawlFinished = true
	}

	if record["msg"] == "url archived" {
		if u, err := url.Parse(record["url"]); err == nil {
			rm.archivedHops[u.Path] = record["hops"]
		}
	}

	if record["level"] == "ERROR" {
		rm.errored = true
	}
}

func (rm *recordMatcher) Assert(t *testing.T) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.crawlFinished {
		t.Error("Zeno did not finish the crawl, the sitemap index cycle may not have terminated")
	}
	if rm.errored {
		t.Error("An error was logged during the test")
	}

	expected := map[string]string{
		"/sitemap_index.xml": "0",
		"/nested_index.xml":  "0",
		"/post-sitemap.xml":  "0",
		"/page-sitemap.xml":  "0",
		"/2026/new-article/": "1",
		"/about/":            "1",
	}
	for path, hops := range expected {
		got, ok := rm.archivedHops[path]
		if !ok {
			t.Errorf("%s was not archived", path)
		} else if got != hops {
			t.Errorf("%s archived at hop %s, expected %s", path, got, hops)
		}
	}

	for path, hops := range rm.archivedHops {
		if _, ok := expected[path]; !ok {
			t.Errorf("%s (hop %s) should not have been archived", path, hops)
		}
	}

	t.Logf("archived URLs and their hops: %v", rm.archivedHops)
}

func (rm *recordMatcher) ShouldStop() bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	return rm.crawlFinished || rm.errored
}

func TestSitemapIndexSameHop(t *testing.T) {
	server := setupServer()
	serverURL := strings.Replace(server.URL, "127.0.0.1", "127.0.0.1.nip.io", 1)
	defer server.Close()
	os.RemoveAll("jobs")
	defer os.RemoveAll("jobs")

	shouldStopCh := make(chan struct{})
	rm := &recordMatcher{archivedHops: make(map[string]string)}
	wg := &sync.WaitGroup{}

	wg.Add(2)

	go e2e.StartHandleLogRecord(t, wg, rm, shouldStopCh)
	go e2e.ExecuteCmdZenoGetURL(t, wg, []string{serverURL + "/sitemap_index.xml"})

	e2e.WaitForGoroutines(t, wg, shouldStopCh)
	rm.Assert(t)
}

// setupServer serves a sitemap index that lists a sitemap and a nested index,
// which itself lists another sitemap and the first index (a cycle).
func setupServer() *httptest.Server {
	recent := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	mux := http.NewServeMux()

	xmlHandler := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/xml; charset=UTF-8")
			fmt.Fprint(w, strings.ReplaceAll(body, "{host}", "http://"+r.Host))
		}
	}
	htmlHandler := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=UTF-8")
			fmt.Fprint(w, body)
		}
	}

	mux.HandleFunc("/sitemap_index.xml", xmlHandler(`<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<sitemap><loc>{host}/post-sitemap.xml</loc><lastmod>`+recent+`</lastmod></sitemap>
	<sitemap><loc>{host}/nested_index.xml</loc></sitemap>
	<sitemap><loc>{host}/post-sitemap-2015.xml</loc><lastmod>2015-12-31T23:59:59+00:00</lastmod></sitemap>
</sitemapindex>`))

	mux.HandleFunc("/nested_index.xml", xmlHandler(`<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<sitemap><loc>{host}/page-sitemap.xml</loc></sitemap>
	<sitemap><loc>{host}/sitemap_index.xml</loc></sitemap>
</sitemapindex>`))

	mux.HandleFunc("/post-sitemap.xml", xmlHandler(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>{host}/2026/new-article/</loc><lastmod>`+recent+`</lastmod></url>
	<url><loc>{host}/2016/old-article/</loc><lastmod>2016-01-01</lastmod></url>
</urlset>`))

	mux.HandleFunc("/post-sitemap-2015.xml", xmlHandler(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>{host}/2015/older-article/</loc></url>
</urlset>`))

	mux.HandleFunc("/page-sitemap.xml", xmlHandler(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>{host}/about/</loc></url>
</urlset>`))

	// Articles link deeper, those pages are at hop 2 and must not be crawled
	mux.HandleFunc("/2026/new-article/", htmlHandler(`<html><body><a href="/2026/new-article/comments/">Comments</a></body></html>`))
	mux.HandleFunc("/about/", htmlHandler(`<html><body><a href="/about/team/">Team</a></body></html>`))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		htmlHandler(`<html><body>`+r.URL.Path+`</body></html>`)(w, r)
	})

	return httptest.NewServer(mux)
}
