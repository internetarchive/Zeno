package extractor

import (
	"bytes"
	"net/http"
	"net/url"
	"testing"

	"github.com/internetarchive/Zeno/pkg/models"
)

// mockReadSeekCloser implements the spooledtempfile.ReadSeekCloser interface
type mockReadSeekCloser struct {
	*bytes.Reader
	name string
}

func (m mockReadSeekCloser) Close() error     { return nil }
func (m mockReadSeekCloser) FileName() string { return m.name }

func TestEbookOutlinkExtractor(t *testing.T) {
	tests := []struct {
		name    string
		xmlData string
		want    bool
	}{
		// add your test cases here
		{"dummy test", "<xml></xml>", true},
	}

	extractor := EbookOutlinkExtractor{}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			URLObj := &models.URL{}
			URLObj.SetRequest(&http.Request{URL: &url.URL{Scheme: "http", Host: "example.com"}})
			URLObj.SetResponse(&http.Response{
				Header: http.Header{
					"Server": []string{"AmazonS3"},
				},
			})

			// Use mockReadSeekCloser instead of spooledtempfile
			tmpFile := mockReadSeekCloser{
				Reader: bytes.NewReader([]byte(tc.xmlData)),
				name:   "test.epub",
			}

			URLObj.SetBody(tmpFile)

			got := extractor.Match(URLObj)
			if got != tc.want {
				t.Errorf("Match() = %v, want %v", got, tc.want)
			}
		})
	}
}
