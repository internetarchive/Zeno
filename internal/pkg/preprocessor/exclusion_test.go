package preprocessor

import (
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/pkg/models"
)

func TestMatchRegexExclusion(t *testing.T) {
	exclusionRegex := []string{
		`(?i)^https?://(www\.)?archive-it\.org.*`,
		`(?i)^https?://(www\.)?x\.com.*`,
		`^https?://127\.0\.`,
		`^https?://192\.168\.`,
		`(?i)https?://[^/]+/wp-admin/`,
		`(?i)^(mailto|sms|tel|data|javascript):`,
	}
	var regexps []*regexp.Regexp
	for _, r := range exclusionRegex {
		re, err := regexp.Compile(r)
		if err != nil {
			t.Fatalf("Failed to compile regex %q: %v", r, err)
		}
		regexps = append(regexps, re)
	}

	tests := []struct {
		name            string
		itemURL         string
		expectedMatched bool
	}{
		{
			name:            "Match localhost IP",
			itemURL:         "http://127.0.0.1/details/testitem",
			expectedMatched: true,
		},
		{
			name:            "Match x.com post with HTTP",
			itemURL:         "HTTPS://x.com:/loukoumi07/status/1922747849671934061",
			expectedMatched: true,
		},
		{
			name:            "Match foo.com wp-admin",
			itemURL:         "https://foo.com/wp-admin/something",
			expectedMatched: true,
		},
		{
			name:            "Match mailto: uppercase link",
			itemURL:         "MAILTO:someone@foo.com",
			expectedMatched: true,
		},
		{
			name:            "Match tel: link",
			itemURL:         "tel:0090567854",
			expectedMatched: true,
		},
		{
			name:            "No match",
			itemURL:         "https://archive.org/details/testitem",
			expectedMatched: false,
		},
		{
			name:            "No match",
			itemURL:         "https://something.org/details/wp-admintestitem",
			expectedMatched: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := models.NewURL(tt.itemURL)
			if err != nil {
				t.Errorf("URL parsing failed %v", err)
			}
			item := models.NewItem(uuid.New().String(), &parsedURL, "")
			got := matchRegexExclusion(regexps, item)
			if got != tt.expectedMatched {
				t.Errorf("Expected match: %v, got: %v", tt.expectedMatched, got)
			}
		})
	}
}
