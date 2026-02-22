package truthsocial

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	_ "embed"

	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

func TestShouldMatchTruthsocialURL(t *testing.T) {

	cases := []struct {
		url      string
		expected bool
	}{
		{"https://truthsocial.com/@realDonaldTrump/posts/115983891481988557", true},
		{"https://truthsocial.com/api/v1/accounts/107780257626128497/statuses?exclude_replies=true&only_replies=false&with_muted=true", true},
		{"https://truthsocial.com/@realDonaldTrump", false},
	}

	for _, c := range cases {
		t.Run(c.url, func(t *testing.T) {
			url, err := models.NewURL(c.url)
			if err != nil {
				t.Fatalf("failed to create URL: %v", err)
			}

			result := TruthsocialExtractor{}.Match(&url)
			if result != c.expected {
				t.Errorf("TruthsocialExtractor{}.Match(%q) = %v; want %v", c.url, result, c.expected)
			}
		})
	}
}

// newItemFromBody creates a *models.Item with the given body, URL and Content-Type for use in tests.
func newItemFromBody(t *testing.T, body []byte, urlStr string, contentType string) *models.Item {
	t.Helper()
	resp := &http.Response{
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBuffer(body)),
		StatusCode: 200,
	}
	resp.Header.Set("Content-Type", contentType)

	newURL, err := models.NewURL(urlStr)
	if err != nil {
		t.Fatalf("failed to create URL: %v", err)
	}
	newURL.SetResponse(resp)

	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write(body)

	newURL.SetBody(spooledTempFile)
	newURL.Parse()
	return models.NewItem(&newURL, "")
}

//go:embed testdata/statuses.json
var rawStatutesJSON []byte

func TestShouldExtractTruthsocialStatusesAPI(t *testing.T) {
	item := newItemFromBody(t, rawStatutesJSON, "https://truthsocial.com/api/v1/accounts/107780257626128497/statuses?exclude_replies=true&only_replies=false&with_muted=true", "application/json")

	assets, outlinks, err := TruthsocialExtractor{}.Extract(item)
	if err != nil {
		t.Fatalf("failed to extract assets: %v", err)
	}

	for _, asset := range assets {
		fmt.Println(asset)
	}
	for _, outlink := range outlinks {
		fmt.Println(outlink)
	}
}
