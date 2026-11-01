package hq

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/internetarchive/Zeno/v2/internal/pkg/log"
	"github.com/internetarchive/Zeno/v2/pkg/models"
	"github.com/internetarchive/gocrawlhq"
)

// The URLs sent to HQ and the URLs HQ returns must be matched with the same string,
// even when the raw URL and its normalized string differ (here because of "/" and ":" in the query).
func TestSeencheckItemMatchesSentURLs(t *testing.T) {
	logger = log.NewFieldedLogger(&log.Fields{"component": "hq.test"})

	const neverSeenURL = "http://example.com/a?x=a/b&y=c:d"
	const seenURL = "http://example.com/b?x=a/b&y=c:d"

	// Fake HQ: everything is new, except seenURL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var URLs []gocrawlhq.URL
		if err := json.NewDecoder(r.Body).Decode(&URLs); err != nil {
			t.Errorf("unable to decode seencheck request: %v", err)
		}

		var notSeen []gocrawlhq.URL
		for _, URL := range URLs {
			if URL.Value != seenURL {
				notSeen = append(notSeen, URL)
			}
		}

		json.NewEncoder(w).Encode(notSeen)
	}))
	defer server.Close()

	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("error parsing server URL: %v", err)
	}

	s := &HQ{
		ctx: context.Background(),
		client: &gocrawlhq.Client{
			HTTPClient:        &http.Client{Transport: &http.Transport{DisableKeepAlives: true}},
			SeencheckEndpoint: endpoint,
		},
	}

	seedURL, err := models.NewURL("http://example.com/")
	if err != nil {
		t.Fatalf("error parsing URL: %v", err)
	}
	seed := models.NewItem(&seedURL, "")

	children := make(map[string]*models.Item)
	for _, raw := range []string{neverSeenURL, seenURL} {
		childURL, err := models.NewURL(raw)
		if err != nil {
			t.Fatalf("error parsing URL: %v", err)
		}
		if childURL.String() == childURL.Raw {
			t.Fatalf("test URL %s must have a raw form different from its string", raw)
		}

		child := models.NewItem(&childURL, "")
		if err := seed.AddChild(child, models.ItemGotChildren); err != nil {
			t.Fatalf("unable to add child: %v", err)
		}
		children[raw] = child
	}

	if err := s.SeencheckItem(seed); err != nil {
		t.Fatalf("seencheck failed: %v", err)
	}

	if status := children[neverSeenURL].GetStatus(); status != models.ItemFresh {
		t.Errorf("expected %s to stay fresh, got status %s", neverSeenURL, status)
	}
	if status := children[seenURL].GetStatus(); status != models.ItemSeen {
		t.Errorf("expected %s to be seen, got status %s", seenURL, status)
	}
}
