package preprocessor

import (
	"strings"
	"testing"

	"github.com/internetarchive/Zeno/v2/internal/pkg/config"
	"github.com/internetarchive/Zeno/v2/pkg/models"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name        string
		rawURL      string
		parentURL   string
		wantErr     bool
		expectedURL string
	}{
		{
			name:        "valid absolute URL",
			rawURL:      "https://example.com/path",
			wantErr:     false,
			expectedURL: "https://example.com/path",
		},
		{
			name:        "valid relative URL with parent",
			rawURL:      "/path",
			parentURL:   "https://example.com",
			wantErr:     false,
			expectedURL: "https://example.com/path",
		},
		{
			name:    "invalid URL",
			rawURL:  "://invalid-url",
			wantErr: true,
		},
		{
			name:        "valid URL without scheme",
			rawURL:      "www.google.com",
			wantErr:     false,
			expectedURL: "http://www.google.com/",
		},
		{
			name:    "FTP url",
			rawURL:  "ftp://ftp.example.com",
			wantErr: true,
		},
		{
			name:        "valid URL with path without scheme",
			rawURL:      "www.google.com/dogs",
			wantErr:     false,
			expectedURL: "http://www.google.com/dogs",
		},
		{
			name:        "URL with leading and trailing quotes",
			rawURL:      `"https://example.com/path"`,
			wantErr:     false,
			expectedURL: "https://example.com/path",
		},
		{
			name:        "relative URL with leading and trailing quotes",
			rawURL:      `'/path'`,
			parentURL:   "https://example.com",
			wantErr:     false,
			expectedURL: "https://example.com/path",
		},
		{
			name:        "relative URL without parent",
			rawURL:      "/path",
			wantErr:     true,
			expectedURL: "",
		},
		{
			name:    "path loop detection - repeated segments",
			rawURL:  "https://example.com/fonts/fonts/fonts/fonts/file.woff2",
			wantErr: true,
		},
		{
			name:    "path loop detection - crawler trap pattern",
			rawURL:  "https://lms.example.com/theme/styles.php/all/DataTables/images/DataTables/fonts/DataTables/fonts/DataTables/images/sort.png",
			wantErr: true,
		},
		{
			name:        "path with acceptable repetition",
			rawURL:      "https://example.com/a/b/a/file.css",
			wantErr:     false,
			expectedURL: "https://example.com/a/b/a/file.css",
		},
		{
			name:    "URL exceeding 4000 characters",
			rawURL:  "https://example.com/" + strings.Repeat("a", 3985),
			wantErr: true,
		},
		{
			name:        "URL at exactly 4000 characters",
			rawURL:      "https://example.com/" + strings.Repeat("a", 3980),
			wantErr:     false,
			expectedURL: "https://example.com/" + strings.Repeat("a", 3980),
		},
	}

	config.Set(&config.Config{MaxURLLength: 4000, MaxSegmentRepetition: 3, MaxSegmentRepetitionThreshold: 2})

	for _, tt := range tests {
		// TODO: add support for nil value of parentURL
		t.Run(tt.name, func(t *testing.T) {
			url := &models.URL{Raw: tt.rawURL}
			var parentURL *models.URL
			if tt.parentURL != "" {
				parentURL = &models.URL{Raw: tt.parentURL}
				parentURL.Parse()
			}
			err := NormalizeURL(url, parentURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("normalizeURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && url.Raw != tt.expectedURL {
				t.Errorf("normalizeURL() got = %v, want %v", url.Raw, tt.expectedURL)
			}
		})
	}
}

// A URL must get the same Raw whether it was found as a relative or an absolute link (e.g. in a
// redirect Location), and Raw must be its String(), because HQ's seencheck uses Raw and the local
// seencheck uses String().
func TestNormalizeURLRelativeAndAbsoluteGiveSameRaw(t *testing.T) {
	config.Set(&config.Config{MaxURLLength: 4000, MaxSegmentRepetition: 3, MaxSegmentRepetitionThreshold: 2})

	parentURL := &models.URL{Raw: "https://example.com/dir/page.html"}
	parentURL.Parse()

	tests := []struct {
		relative    string
		absolute    string
		expectedURL string
	}{
		{"/next?x=a/b&y=c:d", "https://example.com/next?x=a/b&y=c:d", "https://example.com/next?x=a%2Fb&y=c%3Ad"},
		{"next?b=2&a=1 2", "https://example.com/dir/next?b=2&a=1 2", "https://example.com/dir/next?b=2&a=1+2"},
		{"/a?q=100%&c=é", "https://example.com/a?q=100%&c=é", "https://example.com/a?q=100%&c=%C3%A9"},
	}

	for _, tt := range tests {
		relativeURL := &models.URL{Raw: tt.relative}
		if err := NormalizeURL(relativeURL, parentURL); err != nil {
			t.Fatalf("unable to normalize %s: %v", tt.relative, err)
		}
		absoluteURL := &models.URL{Raw: tt.absolute}
		if err := NormalizeURL(absoluteURL, parentURL); err != nil {
			t.Fatalf("unable to normalize %s: %v", tt.absolute, err)
		}

		if relativeURL.Raw != tt.expectedURL {
			t.Errorf("relative %s: got %s, want %s", tt.relative, relativeURL.Raw, tt.expectedURL)
		}
		if absoluteURL.Raw != tt.expectedURL {
			t.Errorf("absolute %s: got %s, want %s", tt.absolute, absoluteURL.Raw, tt.expectedURL)
		}
		if relativeURL.String() != relativeURL.Raw {
			t.Errorf("relative %s: String() %s is not Raw %s", tt.relative, relativeURL.String(), relativeURL.Raw)
		}
	}
}
