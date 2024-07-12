package queue

import (
	"net/url"
	"testing"
)

func TestNewItem(t *testing.T) {
	// Test cases
	testCases := []struct {
		name             string
		url              string
		parentItem       *Item
		itemType         string
		hop              uint8
		id               string
		bypassSeencheck  bool
		expectedHostname string
	}{
		{
			name:             "Basic URL",
			url:              "https://example.com/page",
			parentItem:       nil,
			itemType:         "page",
			hop:              1,
			id:               "",
			bypassSeencheck:  false,
			expectedHostname: "example.com",
		},
		{
			name:             "URL with ID and BypassSeencheck",
			url:              "https://test.org/resource",
			parentItem:       &Item{URL: &url.URL{Host: "parent.com"}},
			itemType:         "resource",
			hop:              2,
			id:               "custom-id",
			bypassSeencheck:  true,
			expectedHostname: "test.org",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse URL
			parsedURL, err := url.Parse(tc.url)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			// Create new item
			item := NewItem(parsedURL, tc.parentItem, tc.itemType, tc.hop, tc.id, tc.bypassSeencheck)

			// Assertions
			if item.URL != parsedURL {
				t.Errorf("Expected URL %v, got %v", parsedURL, item.URL)
			}
			if item.Host != tc.expectedHostname {
				t.Errorf("Expected host %s, got %s", tc.expectedHostname, item.Host)
			}
			if item.Hop != tc.hop {
				t.Errorf("Expected hop %d, got %d", tc.hop, item.Hop)
			}
			if item.ParentItem != tc.parentItem {
				t.Errorf("Expected parent item %v, got %v", tc.parentItem, item.ParentItem)
			}
			if item.Type != tc.itemType {
				t.Errorf("Expected item type %s, got %s", tc.itemType, item.Type)
			}
			if tc.id != "" {
				if item.ID != tc.id {
					t.Errorf("Expected ID %s, got %s", tc.id, item.ID)
				}
			} else {
				if item.ID != "" {
					t.Errorf("Expected empty ID, got %s", item.ID)
				}
			}

			expectedBypassSeencheck := "false"
			if tc.bypassSeencheck {
				expectedBypassSeencheck = "true"
			}
			if item.BypassSeencheck != expectedBypassSeencheck {
				t.Errorf("Expected BypassSeencheck %s, got %s", expectedBypassSeencheck, item.BypassSeencheck)
			}

			// Check that Hash is not empty (we can't predict its exact value)
			if item.Hash == 0 {
				t.Error("Expected non-zero Hash")
			}
		})
	}
}
