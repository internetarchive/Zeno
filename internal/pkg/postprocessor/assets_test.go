package postprocessor

import (
	"bytes"
	_ "embed"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/testutil"
	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

//go:embed testdata/ina_api_response.json
var inaFixture []byte

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

func TestExtractAssets_HydrateItemFixture(t *testing.T) {
	item := testutil.HydrateItem(t, inaFixture)
	assets, _, err := ExtractAssetsOutlinks(item)
	if err != nil {
		t.Fatalf("extract assets from fixture: %v", err)
	}
	// INA API fixture should yield at least resourceUrl, resourceThumbnail, embed URL, uri
	if len(assets) < 1 {
		t.Errorf("expected at least one asset from INA fixture, got %d", len(assets))
	}
	// Sanity: one of the assets should be the resource URL from the fixture body
	found := false
	for _, a := range assets {
		if a != nil && a.Raw == "https://example.com/video.mp4" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected asset https://example.com/video.mp4 in %v", assets)
	}
}

func TestSanitizeAssetsOutlinks(t *testing.T) {
	var err error
	newURL, _ := models.NewURL("http://example.com")
	newItem := models.NewItem(&newURL, "")

	a1, _ := models.NewURL("http://a1.com")
	a2, _ := models.NewURL("mailto:info@archive.org") // must filter out
	a3, _ := models.NewURL("http://example.com")      // equal to item URL, must filter out

	assets := []*models.URL{&a1, &a2, &a3}

	o1, _ := models.NewURL("http://ol1.com")
	o2, _ := models.NewURL("javascript:function(){alert('hi')}") // must filter out
	outlinks := []*models.URL{&o1, &o2}
	assets, outlinks, err = SanitizeAssetsOutlinks(newItem, assets, outlinks, err)

	if err != nil {
		t.Errorf("unexpected error  %v", err)
	}
	if len(assets) != 1 {
		t.Errorf("expected 1 filtered asset, got %d", len(assets))
	}
	if len(outlinks) != 1 {
		t.Errorf("expected 1 filtered outlink, got %d", len(outlinks))
	}
}

// Replace &amp; with & in reddit.com assets to fix Reddit quirk.
func TestRedditAssetQuirks(t *testing.T) {
	var err error
	newURL, _ := models.NewURL("https://reddit.com/")
	newItem := models.NewItem(&newURL, "")

	o1, _ := models.NewURL("http://cnn.com")
	a1, _ := models.NewURL("http://reddit.com/asset?a=1&b=2&amp;c=3")

	assets := []*models.URL{&a1}
	outlinks := []*models.URL{&o1}

	assets, _, err = SanitizeAssetsOutlinks(newItem, assets, outlinks, err)

	if err != nil {
		t.Errorf("unexpected error  %v", err)
	}

	if assets[0].Raw != "http://reddit.com/asset?a=1&b=2&c=3" {
		t.Errorf("expected reddit.com &amp; replacement with & got %s", assets[0].Raw)
	}
}
