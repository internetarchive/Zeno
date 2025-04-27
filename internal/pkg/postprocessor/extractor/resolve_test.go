package extractor

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor"
	"github.com/internetarchive/Zeno/pkg/models"
)

func TestResolveURL(t *testing.T) {
	tests := []struct {
		name          string
		URL           string
		parentURL     string
		base          string
		want          string
		wantBaseUnset bool
		expectErr     bool
	}{
		{
			name:      "Absolute URL passed",
			URL:       "https://otherdomain.com/page",
			parentURL: "https://example.com/index.html",
			base:      "",
			want:      "https://otherdomain.com/page",
		},
		{
			name:      "Relative URL without explicit base",
			URL:       "../resource/page.html",
			parentURL: "https://example.com/section/subsection/index.html",
			base:      "",
			// The parent's directory is https://example.com/section/subsection/ so "../" goes one level up.
			want: "https://example.com/section/resource/page.html",
		},
		{
			name:      "Relative URL with explicit base with trailing slash",
			URL:       "another/page.html",
			parentURL: "https://example.com/section/subsection/index.html",
			base:      "https://example.com/base/",
			want:      "https://example.com/base/another/page.html",
		},
		{
			name:      "Relative URL with explicit base without trailing slash",
			URL:       "another/page.html",
			parentURL: "https://example.com/section/subsection/index.html",
			base:      "https://example.com/base",
			// When the base URL does not end with a slash, it is considered to point to a file.
			// In that case, the directory is used (which is the path minus its last segment).
			// "https://example.com/base" is treated as if its directory were "https://example.com/"
			// so "another/page.html" gets resolved to "https://example.com/another/page.html".
			want: "https://example.com/another/page.html",
		},
		{
			name:      "URL starting with slash with explicit base",
			URL:       "/absolute/path.html",
			parentURL: "https://example.com/section/subsection/index.html",
			base:      "https://example.com/base/",
			// A URL starting with "/" is resolved relative to the root of the domain.
			want: "https://example.com/absolute/path.html",
		},
		{
			name:      "Invalid URL string",
			URL:       "http://%zz",
			parentURL: "https://example.com/index.html",
			base:      "",
			expectErr: true,
		},
		{
			name:          "Invalid base URL with bad URL escape",
			URL:           "page.html",
			parentURL:     "https://example.com/index.html",
			base:          "https://example.com/%zz/",
			want:          "https://example.com/page.html",
			wantBaseUnset: true,
			expectErr:     false,
		},
		{
			name:          "Invalid base URL with disallowed data scheme",
			URL:           "page.html",
			parentURL:     "https://example.com/index.html",
			base:          "data://abcdef/",
			want:          "https://example.com/page.html",
			wantBaseUnset: true,
			expectErr:     false,
		},
		{
			name:      "Empty URL should return base",
			URL:       "",
			parentURL: "https://example.com/index.html",
			base:      "",
			// When urlStr is empty, url.Parse returns an empty URL struct.
			// ResolveReference then returns a copy of the base URL.
			want: "https://example.com/index.html",
		},
		{
			name:      "URL with fragment only",
			URL:       "#section",
			parentURL: "https://example.com/path/page.html",
			base:      "",
			// The fragment is appended to the parent's URL.
			want: "https://example.com/path/page.html#section",
		},
		{
			name:      "Relative URL with dot segment",
			URL:       "./subdir/page.html",
			parentURL: "https://example.com/dir/index.html",
			base:      "",
			// "./" means the same directory as the parent's URL.
			want: "https://example.com/dir/subdir/page.html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := models.NewItem("test", &models.URL{
				Raw: tt.parentURL,
			}, "")

			var doc *goquery.Document
			if tt.base != "" {
				doc = newDocumentWithBaseTag(tt.base)
			} else {
				// empty html
				tt.wantBaseUnset = true
				doc, _ = goquery.NewDocumentFromReader(strings.NewReader(`<html></html>`))
			}

			extractBaseTag(item, doc)

			isBaseUnset := item.GetBase() == nil
			if tt.wantBaseUnset != isBaseUnset {
				t.Errorf("resolveURL() isBaseUnset = %v, test_name = %s, wantBaseUnset %v", isBaseUnset, tt.name, tt.wantBaseUnset)
			}

			preprocessor.NormalizeURL(item.GetURL(), nil)

			got, err := resolveURL(tt.URL, item)
			if (err != nil) != tt.expectErr {
				t.Errorf("resolveURL() error = %v, test_name = %s, expectErr %v", err, tt.name, tt.expectErr)
				return
			}
			if !tt.expectErr && got != tt.want {
				t.Errorf("resolveURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
