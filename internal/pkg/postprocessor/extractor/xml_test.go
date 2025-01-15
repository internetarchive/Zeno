package extractor

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/internetarchive/Zeno/internal/pkg/archiver"
	"github.com/internetarchive/Zeno/pkg/models"
)

func TestXML(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected []string
		hasError bool
	}{
		{
			name: "Valid XML with multiple URLs",
			body: `<?xml version="1.0" encoding="UTF-8"?>
                <urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
                    <url>
                        <loc>https://example.com/page1</loc>
                    </url>
                    <url>
                        <loc>https://example.com/page2</loc>
                    </url>
                </urlset>`,
			expected: []string{
				"http://www.sitemaps.org/schemas/sitemap/0.9",
				"https://example.com/page1",
				"https://example.com/page2",
			},
			hasError: false,
		},
		{
			name:     "Valid XML with no URLs",
			body:     `<?xml version="1.0" encoding="UTF-8"?></urlset>`,
			expected: []string{},
			hasError: false,
		},
		{
			name:     "Invalid XML content",
			body:     `<html><body>Not XML</body></html>`,
			expected: []string{},
			hasError: false,
		},
		{
			name: "XML with nested elements containing URLs",
			body: `<?xml version="1.0" encoding="UTF-8"?>
                <root>
                    <level1>
                        <level2>
                            <url>https://example.com/nested</url>
                        </level2>
                    </level1>
                </root>`,
			expected: []string{
				"https://example.com/nested",
			},
			hasError: false,
		},
		{
			name: "XML with attributes containing URLs",
			body: `<?xml version="1.0" encoding="UTF-8"?>
                <root>
                    <element url="https://example.com/attr"></element>
                </root>`,
			expected: []string{
				"https://example.com/attr",
			},
			hasError: false,
		},
		{
			name: "XML with mixed content",
			body: `<?xml version="1.0" encoding="UTF-8"?>
                <root>
                    <element>Text before URL https://example.com/mixed Text after URL</element>
                </root>`,
			expected: []string{
				"https://example.com/mixed",
			},
			hasError: false,
		},
		{
			name:     "Empty XML content",
			body:     ``,
			expected: []string{},
			hasError: true,
		},
		{
			name: "Large XML content",
			body: `<?xml version="1.0" encoding="UTF-8"?>
                <urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + strings.Repeat(`<url><loc>https://example.com/page</loc></url>`, 1000) + `</urlset>`,
			expected: func() []string {
				var urls = []string{"http://www.sitemaps.org/schemas/sitemap/0.9"}
				for i := 0; i < 1000; i++ {
					urls = append(urls, "https://example.com/page")
				}
				return urls
			}(),
			hasError: false,
		},
		{
			name: "XML with special characters in URLs",
			body: `<?xml version="1.0" encoding="UTF-8"?>
                <urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
                    <url>
                        <loc>https://example.com/page?param=1&amp;other=2</loc>
                    </url>
                </urlset>`,
			expected: []string{
				"http://www.sitemaps.org/schemas/sitemap/0.9",
				"https://example.com/page?param=1&other=2",
			},
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Body: io.NopCloser(bytes.NewBufferString(tt.body)),
			}

			var URL = new(models.URL)
			URL.SetResponse(resp)

			err := archiver.ProcessBody(URL, false, false, 0, os.TempDir())
			if err != nil {
				t.Errorf("ProcessBody() error = %v", err)
			}

			assets, err := XML(URL)
			if (err != nil) != tt.hasError {
				t.Fatalf("XML() error = %v, wantErr %v", err, tt.hasError)
			}

			if len(assets) != len(tt.expected) {
				t.Fatalf("Expected %d assets, got %d", len(tt.expected), len(assets))
			}

			for i, asset := range assets {
				if asset.Raw != tt.expected[i] {
					t.Errorf("Expected asset %s, got %s", tt.expected[i], asset.Raw)
				}
			}
		})
	}
}
