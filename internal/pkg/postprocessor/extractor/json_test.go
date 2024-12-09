package extractor

import (
	"bytes"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/internetarchive/Zeno/pkg/models"
)

func TestJSON(t *testing.T) {
	tests := []struct {
		name     string
		jsonBody string
		wantURLs []*models.URL
		wantErr  bool
	}{
		{
			name:     "Valid JSON with URLs",
			jsonBody: `{"url": "https://example.com", "nested": {"link": "http://test.com"}}`,
			wantURLs: []*models.URL{
				{Raw: "https://example.com"},
				{Raw: "http://test.com"},
			},
			wantErr: false,
		},
		{
			name:     "Invalid JSON",
			jsonBody: `{"url": "https://example.com"`,
			wantURLs: nil,
			wantErr:  true,
		},
		{
			name:     "JSON with no URLs",
			jsonBody: `{"key": "value", "number": 42}`,
			wantURLs: nil,
			wantErr:  false,
		},
		{
			name:     "JSON with URLs in various fields",
			jsonBody: `{"someField": "https://example.com", "otherField": "http://test.com", "nested": {"deepLink": "https://deep.example.com"}}`,
			wantURLs: []*models.URL{
				{Raw: "https://example.com"},
				{Raw: "http://test.com"},
				{Raw: "https://deep.example.com"},
			},
			wantErr: false,
		},
		{
			name:     "JSON with array of URLs",
			jsonBody: `{"links": ["https://example1.com", "https://example2.com"]}`,
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
				Body: io.NopCloser(bytes.NewBufferString(tt.jsonBody)),
			}

			var URL = new(models.URL)
			URL.SetResponse(resp)

			// Consume the response body
			body := bytes.NewBuffer(nil)
			_, err := io.Copy(body, resp.Body)
			if err != nil {
				t.Errorf("unable to read response body: %v", err)
			}

			// Set the body in the URL
			URL.SetBody(bytes.NewReader(body.Bytes()))

			gotURLs, err := JSON(URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("JSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Sort both slices before comparison
			sortURLs(gotURLs)
			sortURLs(tt.wantURLs)

			if !reflect.DeepEqual(gotURLs, tt.wantURLs) {
				t.Errorf("JSON() gotURLs = %v, want %v", gotURLs, tt.wantURLs)
			}
		})
	}
}
