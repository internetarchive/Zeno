package archiver

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeRoundTripper counts how many times RoundTrip is called
type fakeRoundTripper struct {
	calls int32
}

func (f *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	atomic.AddInt32(&f.calls, 1)

	body := io.NopCloser(strings.NewReader("hello"))
	return &http.Response{
		StatusCode: 200,
		Body:       body,
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func TestHTTPCacheTransport_Deduplication(t *testing.T) {
	// Initialize fake transport
	fake := &fakeRoundTripper{}

	// Wrap it with our cache transport
	cacheTransport := NewHTTPCacheTransport(fake)

	client := &http.Client{Transport: cacheTransport}

	// First request to URL1
	req1, _ := http.NewRequest("GET", "http://example.com/page1", nil)
	client.Do(req1)

	// Second request to same URL1 (should be skipped by cache)
	req2, _ := http.NewRequest("GET", "http://example.com/page1", nil)
	client.Do(req2)

	// Request to a new URL2 (should hit the network)
	req3, _ := http.NewRequest("GET", "http://example.com/page2", nil)
	client.Do(req3)

	if fake.calls != 2 {
		t.Errorf("expected 2 actual network calls, got %d", fake.calls)
	}
}
