package hqr

import (
	"testing"

	"github.com/internetarchive/gocrawlhq"
)

func TestEnsureAllURLsUnique(t *testing.T) {
	tests := []struct {
		name      string
		URLs      []gocrawlhq.URL
		wantError bool
	}{
		{
			name: "unique URLs",
			URLs: []gocrawlhq.URL{
				{ID: "1", Value: "http://example.com/1"},
				{ID: "2", Value: "http://example.com/2"},
				{ID: "3", Value: "http://example.com/3"},
			},
			wantError: false,
		},
		{
			name: "duplicate URLs",
			URLs: []gocrawlhq.URL{
				{ID: "1", Value: "http://example.com/1"},
				{ID: "2", Value: "http://example.com/2"},
				{ID: "1", Value: "http://example.com/1"},
			},
			wantError: true,
		},
		{
			name:      "empty URLs",
			URLs:      []gocrawlhq.URL{},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureAllURLsUnique(tt.URLs)
			if (err != nil) != tt.wantError {
				t.Errorf("ensureAllURLsUnique() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
