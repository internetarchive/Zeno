package extractor

import (
	"net/http"
	"testing"

	"github.com/internetarchive/Zeno/pkg/models"
)

// TestIsS3 checks the Server header for known S3 strings.
func TestIsS3(t *testing.T) {
	tests := []struct {
		name   string
		server string
		want   bool
	}{
		{"AmazonS3", "AmazonS3", true},
		{"WasabiS3", "WasabiS3", true},
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
					"Server": []string{tt.server},
				},
			})

			got := IsS3(URLObj)
			if got != tt.want {
				t.Errorf("IsS3(server=%q) = %v, want %v", tt.server, got, tt.want)
			}
		})
	}
}
