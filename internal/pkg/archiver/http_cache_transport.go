package archiver

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// cacheEntry stores info about a previous response
type cacheEntry struct {
	etag         string
	lastModified string
	timestamp    time.Time
}

// HTTPCacheTransport wraps an existing RoundTripper and implements deduplication
type HTTPCacheTransport struct {
	Transport http.RoundTripper
	mu        sync.Mutex
	cache     map[string]*cacheEntry
}

// NewHTTPCacheTransport initializes the cache wrapper
func NewHTTPCacheTransport(base http.RoundTripper) *HTTPCacheTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &HTTPCacheTransport{
		Transport: base,
		cache:     make(map[string]*cacheEntry),
	}
}

// RoundTrip implements the http.RoundTripper interface with caching/deduplication
func (c *HTTPCacheTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	key := req.URL.String()

	c.mu.Lock()
	entry, ok := c.cache[key]
	c.mu.Unlock()

	// If we have a cache entry, return 304 without hitting network
	if ok && !entry.shouldRefresh() {
		return &http.Response{
			StatusCode: http.StatusNotModified,
			Header:     make(http.Header),
			Request:    req,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	}

	// Add conditional headers if we have a cache entry
	if ok {
		if entry.etag != "" {
			req.Header.Set("If-None-Match", entry.etag)
		}
		if entry.lastModified != "" {
			req.Header.Set("If-Modified-Since", entry.lastModified)
		}
	}

	// Perform the actual request
	resp, err := c.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Update cache if response is cacheable
	if resp.StatusCode == http.StatusOK {
		c.mu.Lock()
		c.cache[key] = &cacheEntry{
			etag:         resp.Header.Get("ETag"),
			lastModified: resp.Header.Get("Last-Modified"),
			timestamp:    time.Now(),
		}
		c.mu.Unlock()
	}

	return resp, nil
}

// shouldRefresh decides if cache entry is stale
func (e *cacheEntry) shouldRefresh() bool {
	return false
}
