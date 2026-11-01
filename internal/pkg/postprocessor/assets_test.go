package postprocessor

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/internetarchive/Zeno/v2/internal/pkg/config"
	"github.com/internetarchive/Zeno/v2/pkg/models"
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

	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", t.TempDir(), 2048, false, -1)
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

// The item's own URL is removed from its assets however it is written in the page.
func TestSanitizeAssetsOutlinks_ItemURLWrittenDifferently(t *testing.T) {
	newURL, _ := models.NewURL("http://example.com/doc.html?b=x/y&a=1")
	newItem := models.NewItem(&newURL, "")

	a1, _ := models.NewURL("http://example.com/doc.html?b=x/y&a=1#top")
	a2, _ := models.NewURL("http://example.com/doc.html?b=x%2Fy&a=1")
	a3, _ := models.NewURL("http://example.com/doc.html?a=1&b=x/y") // another order is another URL
	a4, _ := models.NewURL("http://example.com/img.png")

	assets, _, err := SanitizeAssetsOutlinks(newItem, []*models.URL{&a1, &a2, &a3, &a4}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if len(assets) != 2 || assets[0] != &a3 || assets[1] != &a4 {
		var raws []string
		for _, asset := range assets {
			raws = append(raws, asset.Raw)
		}
		t.Errorf("expected only %s and %s as assets, got %v", a3.Raw, a4.Raw, raws)
	}

	encodedURL, _ := models.NewURL("http://example.com/caf%C3%A9.html")
	encodedItem := models.NewItem(&encodedURL, "")
	e1, _ := models.NewURL("http://example.com/caf%C3%A9.html#top")
	e2, _ := models.NewURL("http://example.com/caf%C3%A8.html")

	assets, _, err = SanitizeAssetsOutlinks(encodedItem, []*models.URL{&e1, &e2}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(assets) != 1 || assets[0] != &e2 {
		t.Errorf("expected only %s as asset, got %d assets", e2.Raw, len(assets))
	}

	// An encoded slash is not a path separator
	slashURL, _ := models.NewURL("https://example.com/files/a%2Fb.html")
	slashItem := models.NewItem(&slashURL, "")
	s1, _ := models.NewURL("https://example.com/files/a%2Fb.html#top")
	s2, _ := models.NewURL("https://example.com/files/a/b.html")

	assets, _, err = SanitizeAssetsOutlinks(slashItem, []*models.URL{&s1, &s2}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(assets) != 1 || assets[0] != &s2 {
		t.Errorf("expected only %s as asset, got %d assets", s2.Raw, len(assets))
	}

	rootURL, _ := models.NewURL("http://example.com")
	rootItem := models.NewItem(&rootURL, "")
	r1, _ := models.NewURL("http://example.com#top")
	r2, _ := models.NewURL("http://example.com/")

	assets, _, err = SanitizeAssetsOutlinks(rootItem, []*models.URL{&r1, &r2}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(assets) != 1 || assets[0] != &r2 {
		t.Errorf("expected only %s as asset, got %d assets", r2.Raw, len(assets))
	}
}

func TestRawPath(t *testing.T) {
	tests := []struct {
		rawURL string
		path   string
		ok     bool
	}{
		{"http://example.com/dir/doc.html?a=/b#c/d", "/dir/doc.html", true},
		{"http://example.com/files/a%2Fb.html#top", "/files/a%2Fb.html", true},
		{"http://example.com/", "/", true},
		{"http://example.com", "", true},
		{"http://example.com?a=/b", "", true},
		{"//example.com/doc.html", "", false},
		{"mw-data:TemplateStyles:r1156832818", "", false},
		{"page.html?u=http://example.com/a", "", false},
	}

	for _, tt := range tests {
		path, ok := rawPath(tt.rawURL)
		if path != tt.path || ok != tt.ok {
			t.Errorf("rawPath(%q) = %q, %v; want %q, %v", tt.rawURL, path, ok, tt.path, tt.ok)
		}
	}
}

// A reddit.com asset that can't be unescaped is kept as is, it used to loop forever.
func TestRedditAssetUnescapable(t *testing.T) {
	newURL, _ := models.NewURL("https://reddit.com/")
	newItem := models.NewItem(&newURL, "")

	a1, _ := models.NewURL("http://reddit.com/asset?q=100%")
	a2, _ := models.NewURL("http://reddit.com/asset?a=1&amp;b=2")

	done := make(chan []*models.URL)
	go func() {
		assets, _, _ := SanitizeAssetsOutlinks(newItem, []*models.URL{&a1, &a2}, nil, nil)
		done <- assets
	}()

	select {
	case assets := <-done:
		if len(assets) != 2 || assets[0].Raw != "http://reddit.com/asset?q=100%" || assets[1].Raw != "http://reddit.com/asset?a=1&b=2" {
			t.Errorf("unexpected assets %v and %v", assets[0].Raw, assets[1].Raw)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SanitizeAssetsOutlinks did not return")
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
