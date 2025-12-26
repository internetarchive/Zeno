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
		t.Fatalf("failed to create URL: %v", err)
	}
	newURL.SetResponse(resp)

	spf := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	_, _ = spf.Write([]byte(html))

	newURL.SetBody(spf)
	newURL.Parse()
	item := models.NewItem(&newURL, "")

	assets, outlinks, err := ExtractAssetsOutlinks(item)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(assets) != 2 {
		t.Errorf("expected 2 assets, got %d", len(assets))
	}
	if assets[0].Raw != "http://example.com/img.png" && assets[1].Raw != "http://example.com/img.png" {
		t.Errorf("asset extraction failed for img.png, got %+v", assets)
	}
	if assets[0].Raw != "http://example.com/style.css" && assets[1].Raw != "http://example.com/style.css" {
		t.Errorf("asset extraction failed for style.css, got %+v", assets)
	}
	if len(outlinks) != 0 {
		t.Errorf("expected no outlinks, got %d", len(outlinks))
	}
}

func TestSanitizeAssetsOutlinks(t *testing.T) {
	newURL, _ := models.NewURL("http://example.com")
	newItem := models.NewItem(&newURL, "")

	a1, _ := models.NewURL("http://a1.com")
	a2, _ := models.NewURL("mailto:info@archive.org") // must filter out
	a3, _ := models.NewURL("http://example.com")      // equal to item URL, must filter out

	assets := []*models.URL{&a1, &a2, &a3}

	o1, _ := models.NewURL("http://ol1.com")
	o2, _ := models.NewURL("javascript:function(){alert('hi')}") // must filter out
	outlinks := []*models.URL{&o1, &o2}

	var err error
	assets, outlinks, err = SanitizeAssetsOutlinks(newItem, assets, outlinks, err)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(assets) != 1 {
		t.Errorf("expected 1 filtered asset, got %d", len(assets))
	}
	if len(outlinks) != 1 {
		t.Errorf("expected 1 filtered outlink, got %d", len(outlinks))
	}
}

func TestRedditAssetQuirks(t *testing.T) {
	newURL, _ := models.NewURL("https://reddit.com/")
	newItem := models.NewItem(&newURL, "")

	o1, _ := models.NewURL("http://cnn.com")
	a1, _ := models.NewURL("http://reddit.com/asset?a=1&b=2&amp;c=3")

	assets := []*models.URL{&a1}
	outlinks := []*models.URL{&o1}

	var err error
	assets, _, err = SanitizeAssetsOutlinks(newItem, assets, outlinks, err)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if assets[0].Raw != "http://reddit.com/asset?a=1&b=2&c=3" {
		t.Errorf("expected reddit.com &amp; replacement with &, got %s", assets[0].Raw)
	}
}
