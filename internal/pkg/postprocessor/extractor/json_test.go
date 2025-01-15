package extractor

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/internetarchive/Zeno/internal/pkg/archiver"
	"github.com/internetarchive/Zeno/pkg/models"
)

func TestJSON(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantURLs []*models.URL
		wantErr  bool
	}{
		{
			name: "Valid JSON with URLs",
			body: `{"url": "https://example.com", "nested": {"link": "http://test.com"}}`,
			wantURLs: []*models.URL{
				{Raw: "https://example.com"},
				{Raw: "http://test.com"},
			},
			wantErr: false,
		},
		{
			name:     "Invalid JSON",
			body:     `{"url": "https://example.com"`,
			wantURLs: nil,
			wantErr:  true,
		},
		{
			name:     "JSON with no URLs",
			body:     `{"key": "value", "number": 42}`,
			wantURLs: nil,
			wantErr:  false,
		},
		{
			name: "JSON with URLs in various fields",
			body: `{"someField": "https://example.com", "otherField": "http://test.com", "nested": {"deepLink": "https://deep.example.com"}}`,
			wantURLs: []*models.URL{
				{Raw: "https://example.com"},
				{Raw: "http://test.com"},
				{Raw: "https://deep.example.com"},
			},
			wantErr: false,
		},
		{
			name: "JSON with array of URLs",
			body: `{"links": ["https://example1.com", "https://example2.com"]}`,
			wantURLs: []*models.URL{
				{Raw: "https://example1.com"},
				{Raw: "https://example2.com"},
			},
			wantErr: false,
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

			gotURLs, err := JSON(URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("JSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Sort both slices before comparison
			sortURLs(gotURLs)
			sortURLs(tt.wantURLs)

			if len(gotURLs) != len(tt.wantURLs) {
				t.Fatalf("Expected %d URLs, got %d", len(tt.wantURLs), len(gotURLs))
			}

			for i := range gotURLs {
				if gotURLs[i].Raw != tt.wantURLs[i].Raw {
					t.Errorf("Expected URL %s, got %s", tt.wantURLs[i].Raw, gotURLs[i].Raw)
				}
			}
		})
	}
}
