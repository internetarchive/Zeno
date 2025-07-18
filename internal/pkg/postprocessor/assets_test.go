package postprocessor

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

func TestExtractAssets_HTML(t *testing.T) {
	config.Set(&config.Config{})
	config.Get().DisableHTMLTag = []string{} // initialize as empty slice

	// Create a mock response with a minimal HTML body
	html := `<html><head><link href="style.css"></head><body><img src="img.png"></body></html>`
	resp := &http.Response{
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(html)),
		StatusCode: 200,
	}
	resp.Header.Set("Content-Type", "text/html")

	newURL, err := models.NewURL("http://example.com")
	if err != nil {
		panic(err)
	}
	newURL.SetResponse(resp)

	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write([]byte(html))

	newURL.SetBody(spooledTempFile)
	newURL.Parse()
	item := models.NewItem(&newURL, "")

	assets, outlinks, err := ExtractAssetsOutlinks(item)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Basic assertions
	if len(assets) != 2 {
		t.Errorf("expected assets, got %d", len(assets))
	}
	if assets[0].Raw != "http://example.com/img.png" {
		t.Errorf("asset extraction failed for http://example.com/img.png")

	}
	if assets[1].Raw != "http://example.com/style.css" {
		t.Errorf("asset extraction failed for http://example.com/style.css")
	}
	if len(outlinks) != 0 {
		t.Errorf("expected no outlinks, got %d", len(outlinks))
	}
}
