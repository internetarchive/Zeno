package testutil

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

// NewItemFromBody creates a *models.Item with the given body, URL and Content-Type for use in tests.
func NewItemFromBody(t *testing.T, body []byte, urlStr string, contentType string) *models.Item {
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
