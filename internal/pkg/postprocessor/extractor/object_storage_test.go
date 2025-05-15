package extractor

import (
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

func buildTestObjectStorageURLObj(selfURL, xmlBody string, respHeader http.Header) *models.URL {
	urlURL, err := url.Parse(selfURL)
	if err != nil {
		panic(err)
	}

	URL := &models.URL{}
	URL.SetRequest(&http.Request{URL: urlURL})

	URL.SetResponse(&http.Response{
		Header: respHeader,
	})

	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write([]byte(xmlBody))

	URL.SetBody(spooledTempFile)
	URL.Parse()
	return URL
}

// TestIsObjectStorage checks the Server header for known OSS Server strings.
func TestIsObjectStorage(t *testing.T) {
	tests := []struct {
		name   string
		server string
		want   bool
	}{
		{"AmazonS3", "AmazonS3", true},
		{"WasabiS3", "WasabiS3", true},
		{"Azurite", "Azurite-Blob/3.34.0", true},
		{"AliyunOSS", "AliyunOSS", true},
		{"No match", "Apache", false},
		{"Partial match", "Amazon", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a *models.URL with the response Server header set
			URLObj := &models.URL{}

			URLObj.SetResponse(&http.Response{
				Header: http.Header{
					"Server":       []string{tt.server},
					"Content-Type": []string{"text/xml"},
				},
			})

			got := IsObjectStorage(URLObj)
			if got != tt.want {
				t.Errorf("IsObjectStorage(server=%q) = %v, want %v", tt.server, got, tt.want)
			}
		})
	}
}

func TestObjectStorage(t *testing.T) {
	t.Run("Unknown object storage server", func(t *testing.T) {
		xmlBody := `<Placeholder>XML</Placeholder>`

		URLObj := buildTestObjectStorageURLObj("https://example.com/", xmlBody, http.Header{"Server": []string{"NotAnOSS"}})
		_, err := ObjectStorage(URLObj)

		if err == nil {
			t.Fatalf("expected error for unknown object storage server, got none")
		}

		if err.Error() != "unknown object storage server: NotAnOSS" {
			t.Fatalf("expected error 'unknown object storage server: NotAnOSS', got %v", err)
		}
	})
}
