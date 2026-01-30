package extractor

import (
	"bytes"
	_ "embed"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	generalarchiver "github.com/internetarchive/Zeno/internal/pkg/archiver/general"
	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

//go:embed testdata/rss2.0.xml
var rss2_0XML string

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
			name:     "Invalid XML content (HTML rejected)",
			body:     `<html><body>Not XML</body></html>`,
			expected: []string{},
			hasError: false,
		},
		{
			name: "Valid XML with multiple URLs with whitespace",
			body: `   \t\n   \t\n  <?xml version="1.0" encoding="UTF-8"?>
                <urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
                    <url>
                        <loc>https://example.com/page1</loc>
                    </url>
                    <url>
                        <loc>https://example.com/page2</loc>
                    </url>
                </urlset> \t\n \t\n `,
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
				for range 1000 {
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
		{
			name: "XML RSS",
			body: rss2_0XML,
			expected: func() []string {
				v := make([]string, 213)
				v[0] = "https://blog.archive.org/wp-content/uploads/2023/03/ia-logo-sq-150x150.png"             // image::url
				v[11] = "https://blog.archive.org/wp-content/uploads/2025/03/Vanishing-Culture-Prelinger-3.png" // <a> href in description::CDATA
				v[212] = "https://blog.archive.org/2025/02/06/update-on-the-2024-2025-end-of-term-web-archive/feed/"
				return v

			}(),
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Body:   io.NopCloser(bytes.NewBufferString(tt.body)),
				Header: make(http.Header),
			}
			resp.Header.Set("Content-Type", "application/xml")

			var URL = new(models.URL)
			URL.SetResponse(resp)

			err := generalarchiver.ProcessBody(URL, false, false, 0, os.TempDir(), nil)
			if err != nil {
				t.Errorf("ProcessBody() error = %v", err)
			}

			assets, outlinks, err := XML(URL)

			URLs := append(assets, outlinks...)

			if (err != nil) != tt.hasError {
				t.Fatalf("XML() error = %v, wantErr %v", err, tt.hasError)
			}

			if len(URLs) != len(tt.expected) {
				t.Fatalf("Expected %d assets, got %d", len(tt.expected), len(URLs))
			}

			for i, URL := range URLs {
				if URL.Raw != tt.expected[i] && tt.expected[i] != "" {
					t.Errorf("Expected asset %s, index %d, got %s", tt.expected[i], i, URL.Raw)
				}
			}
		})
	}
}

// TestSitemapXMLOutlinkExtractor covers multiple scenarios.
func TestSitemapXMLOutlinkExtractor(t *testing.T) {
	tests := []struct {
		name    string
		xmlData string
		want    bool
	}{
		{
			name: "Valid sitemap XML",
			xmlData: `<?xml version="1.0" encoding="UTF-8"?>
				<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
					<url>
						<loc>https://example.com/page1</loc>
					</url>
				</urlset>`,
			want: true,
		},
		{
			name: "Invalid sitemap XML",
			xmlData: `<?xml version="1.0" encoding="UTF-8"?>
				<root>
					<element>Not a sitemap</element>
				</root>`,
			want: false,
		},
		{
			name: "Sitemap XML with comment containing marker",
			xmlData: `<?xml version="1.0" encoding="UTF-8"?>
				<!-- http://www.sitemaps.org/schemas/sitemap/0.9 -->
				<root>
					<element>Not a sitemap</element>
				</root>`,
			want: true,
		},
		{
			name: "Sitemap XML with directive containing marker",
			xmlData: `<?xml version="1.0" encoding="UTF-8"?>
				<!DOCTYPE root SYSTEM "http://www.sitemaps.org/schemas/sitemap/0.9">
				<root>
					<element>Not a sitemap</element>
				</root>`,
			want: true,
		},
		{
			name: "Sitemap XML with processing instruction containing marker",
			xmlData: `<?xml version="1.0" encoding="UTF-8"?>
				<?xml-stylesheet type="text/xsl" href="http://www.sitemaps.org/schemas/sitemap/0.9"?>
				<root>
					<element>Not a sitemap</element>
				</root>`,
			want: true,
		},
		{
			name: "Sitemap XML with nested elements containing marker",
			xmlData: `<?xml version="1.0" encoding="UTF-8"?>
				<root xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
					<element>Not a sitemap</element>
				</root>`,
			want: true,
		},
		{
			name: "Sitemap XML with attributes containing marker",
			xmlData: `<?xml version="1.0" encoding="UTF-8"?>
				<root>
					<element xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">Not a sitemap</element>
				</root>`,
			want: true,
		},
		{
			name:    "Empty XML content",
			xmlData: ``,
			want:    false,
		},
		{
			name: "Large sitemap XML content",
			xmlData: `<?xml version="1.0" encoding="UTF-8"?>
				<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + strings.Repeat(`<url><loc>https://example.com/page</loc></url>`, 1000) + `</urlset>`,
			want: true,
		},
		{
			name: "Sitemap XML with special characters in namespace",
			xmlData: `<?xml version="1.0" encoding="UTF-8"?>
				<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9?param=1&amp;other=2">
					<url>
						<loc>https://example.com/page</loc>
					</url>
				</urlset>`,
			want: true,
		},
		{
			name: "Sitemap XML with special characters in URLs",
			xmlData: `<?xml version="1.0" encoding="UTF-8"?>
				<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
					<url>
						<loc>https://example.com/page?param=1&amp;other=2</loc>
					</url>
				</urlset>`,
			want: true,
		},
	}

	extractor := SitemapXMLOutlinkExtractor{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Construct a minimal FakeURL with your test data as body
			URLObj := &models.URL{}
			URLObj.SetRequest(&http.Request{URL: &url.URL{Scheme: "http", Host: "example.com"}})

			// Likewise, set the HTTP response header using SetResponse.
			// We want to simulate an S3 server for these tests.
			URLObj.SetResponse(&http.Response{
				Header: http.Header{
					"Content-Type": []string{"application/xml"},
					"Server":       []string{"AmazonS3"},
				},
			})

			spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
			spooledTempFile.Write([]byte(tc.xmlData))

			URLObj.SetBody(spooledTempFile)

			got := extractor.Match(URLObj)
			if got != tc.want {
				t.Errorf("IsSitemapXML(%q) = %v, want %v", tc.xmlData, got, tc.want)
			}
		})
	}
}
