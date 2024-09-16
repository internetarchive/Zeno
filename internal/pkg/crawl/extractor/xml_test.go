package extractor

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
)

func TestXML(t *testing.T) {
	tests := []struct {
		name          string
		xmlBody       string
		wantURLs      []*url.URL
		wantURLsCount int
		wantErr       bool
		sitemap       bool
	}{
		{
			name: "Valid XML with URLs",
			xmlBody: `
				<root>
					<item>http://example.com</item>
					<nested>
						<url>https://example.org</url>
					</nested>
					<noturl>just some text</noturl>
				</root>`,
			wantURLs: []*url.URL{
				{Scheme: "http", Host: "example.com"},
				{Scheme: "https", Host: "example.org"},
			},
			sitemap: false,
			wantErr: false,
		},
		{
			name:     "Empty XML",
			xmlBody:  `<root></root>`,
			wantURLs: nil,
			wantErr:  false,
			sitemap:  false,
		},
		{
			name:     "Invalid XML",
			xmlBody:  `<root><unclosed>`,
			wantURLs: nil,
			wantErr:  true,
			sitemap:  false,
		},
		{
			name: "XML with invalid URL",
			xmlBody: `
				<root>
					<item>http://example.com</item>
					<item>not a valid url</item>
				</root>`,
			wantURLs: []*url.URL{
				{Scheme: "http", Host: "example.com"},
			},
			wantErr: false,
			sitemap: false,
		},
		{
			name:          "Huge sitemap",
			xmlBody:       loadTestFile(t, "xml_test_sitemap.xml"),
			wantURLsCount: 100002,
			wantErr:       false,
			sitemap:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Body: io.NopCloser(bytes.NewBufferString(tt.xmlBody)),
			}

			gotURLs, sitemap, err := XML(resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("XML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantURLsCount != 0 {
				if len(gotURLs) != tt.wantURLsCount {
					t.Errorf("XML() gotURLs count = %v, want %v", len(gotURLs), tt.wantURLsCount)
				}
			}

			if tt.wantURLs != nil {
				if !compareURLs(gotURLs, tt.wantURLs) {
					t.Errorf("XML() gotURLs = %v, want %v", gotURLs, tt.wantURLs)
				}
			}

			if tt.sitemap != sitemap {
				t.Errorf("XML() sitemap = %v, want %v", sitemap, tt.sitemap)
			}
		})
	}
}

func loadTestFile(t *testing.T, path string) string {
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("openFile() error = %v", err)
	}

	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("readFile() error = %v", err)
	}

	return string(b)
}

func TestXMLBodyReadError(t *testing.T) {
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader([]byte{})), // Empty reader to simulate EOF
	}
	resp.Body.Close() // Close the body to simulate a read error

	_, _, err := XML(resp)
	if err == nil {
		t.Errorf("XML() expected error, got nil")
	}
}
