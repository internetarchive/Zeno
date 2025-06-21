package extractor

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/ImVexed/fasturl"
	"github.com/davecgh/go-spew/spew"
	"github.com/internetarchive/Zeno/internal/pkg/archiver"
	"github.com/internetarchive/Zeno/pkg/models"
)

func TestJSON(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantAssets   []*models.URL
		wantOutlinks []*models.URL
		wantErr      bool
	}{
		{
			name: "JSON in script tag",
			body: `{"ajaxurl":"http:\/\/fakeurl.invalid\/wp-admin\/admin-ajax.php","days":"Days","hours":"Hours","minutes":"Minutes","seconds":"Seconds","ajax_nonce":"c35d389da5"}`,
			wantAssets: []*models.URL{
				{Raw: "http://fakeurl.invalid/wp-admin/admin-ajax.php"},
			},
			wantErr: false,
		},
		{
			name: "Valid JSON with URLs",
			body: `{"url": "https://example.com", "nested": {"link": "http://test.com"}}`,
			wantOutlinks: []*models.URL{
				{Raw: "https://example.com"},
				{Raw: "http://test.com"},
			},
			wantErr: false,
		},
		{
			name:    "Invalid JSON",
			body:    `{"url": "https://example.com"`,
			wantErr: true,
		},
		{
			name:    "JSON with no URLs",
			body:    `{"key": "value", "number": 42}`,
			wantErr: false,
		},
		{
			name: "JSON with URLs in various fields",
			body: `{"someField": "https://example.com", "otherField": "http://test.com", "nested": {"deepLink": "https://deep.example.com"}}`,
			wantOutlinks: []*models.URL{
				{Raw: "https://example.com"},
				{Raw: "http://test.com"},
				{Raw: "https://deep.example.com"},
			},
			wantErr: false,
		},
		{
			name: "JSON with array of URLs",
			body: `{"links": ["https://example1.com", "https://example2.com"]}`,
			wantOutlinks: []*models.URL{
				{Raw: "https://example1.com"},
				{Raw: "https://example2.com"},
			},
			wantErr: false,
		},
		{
			name: "JSON in JSON string",
			body: `{"dic": "{\"url\": \"https://example1.com\"}", "array": "[\"https://example2.com\"]"}`,
			wantOutlinks: []*models.URL{
				{Raw: "https://example1.com"},
				{Raw: "https://example2.com"},
			},
			wantErr: false,
		},
		{
			name: "URLs in text fields",
			body: `{"body": "Check this link https://example.com and also http://test.com"}`,
			wantOutlinks: []*models.URL{
				{Raw: "https://example.com"},
				{Raw: "http://test.com"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Body:   io.NopCloser(bytes.NewBufferString(tt.body)),
				Header: make(http.Header),
			}
			resp.Header.Set("Content-Type", "application/json")

			var URL = new(models.URL)
			URL.SetResponse(resp)

			err := archiver.ProcessBody(URL, false, false, 0, os.TempDir())
			if err != nil {
				t.Errorf("ProcessBody() error = %v", err)
			}

			assets, outlinks, err := JSON(URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("JSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Sort both slices before comparison
			sortURLs(assets)
			sortURLs(tt.wantAssets)
			sortURLs(outlinks)
			sortURLs(tt.wantOutlinks)

			if len(assets) != len(tt.wantAssets) {
				t.Fatalf("Expected %d Assets, got %d", len(tt.wantAssets), len(assets))
			}

			for i := range assets {
				if assets[i].Raw != tt.wantAssets[i].Raw {
					t.Errorf("Expected URL %s, got %s", tt.wantAssets[i].Raw, assets[i].Raw)
				}
			}

			if len(outlinks) != len(tt.wantOutlinks) {
				t.Fatalf("Expected %d Outlinks, got %d", len(tt.wantOutlinks), len(outlinks))
			}

			for i := range outlinks {
				if outlinks[i].Raw != tt.wantOutlinks[i].Raw {
					t.Errorf("Expected Outlink %s, got %s", tt.wantOutlinks[i].Raw, outlinks[i].Raw)
				}
			}
		})
	}
}

func TestIsValidURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"Valid URL", "https://example.com", true},
		{"URL with spaces", "http://example.com/some path", true},
		{"URL with special characters", "http://example.com/some?query=param&another=param", true},
		{"hostname with path", "example.com/path/to/resource", true},
		{"Invalid URL", "not a url", false},
		{"Empty String", "", false},
		{"A Word", "Days", false},
		{"Hostname with query", "example.com?query=param", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidURL(tt.url)
			if result != tt.expected {
				t.Errorf("isValidURL(%q) = %v; want %v", tt.url, result, tt.expected)
				spew.Dump(fasturl.ParseURL(tt.url))
			}
		})
	}
}
