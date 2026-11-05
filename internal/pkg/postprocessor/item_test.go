package postprocessor

import (
	"context"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/internetarchive/Zeno/v2/internal/pkg/config"
	"github.com/internetarchive/Zeno/v2/internal/pkg/postprocessor/domainscrawl"
	"github.com/internetarchive/Zeno/v2/pkg/models"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

func newArchivedHTMLItem(t *testing.T, rawURL, body string) *models.Item {
	t.Helper()

	newURL, err := models.NewURL(rawURL)
	if err != nil {
		t.Fatalf("failed to create URL: %v", err)
	}

	resp := &http.Response{
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		StatusCode: 200,
	}
	resp.Header.Set("Content-Type", "text/html")
	newURL.SetResponse(resp)

	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write([]byte(body))
	t.Cleanup(func() { spooledTempFile.Close() })
	newURL.SetBody(spooledTempFile)

	item := models.NewItem(&newURL, "")
	item.SetStatus(models.ItemArchived)
	return item
}

func TestPostprocessItemDisableAssetsCapture(t *testing.T) {
	page := `<html><body>
<img src="https://example.com/photo.jpg">
<a href="https://example.com/article">article</a>
</body></html>`

	for _, disableAssets := range []bool{false, true} {
		previous := config.Get()
		config.Set(&config.Config{MaxHops: 1, DisableAssetsCapture: disableAssets})

		item := newArchivedHTMLItem(t, "https://example.com/", page)
		outlinks := postprocessItem(context.Background(), item)

		config.Set(previous)

		var raws []string
		for _, outlink := range outlinks {
			raws = append(raws, outlink.GetURL().Raw)
		}
		if !slices.Contains(raws, "https://example.com/article") {
			t.Errorf("disableAssets=%v: expected https://example.com/article in outlinks, got %v", disableAssets, raws)
		}

		if disableAssets && item.HasChildren() {
			t.Errorf("disableAssets=true: expected no assets, got %d children", len(item.GetChildren()))
		}
		if !disableAssets && !item.HasChildren() {
			t.Errorf("disableAssets=false: expected the image as an asset, got no children")
		}
	}
}

// In a domains crawl, HTML extracted as an asset (e.g. from a canonical <link>) is
// only parsed for assets of its own when the domains crawl matches it: off-site
// HTML assets keep the usual limit, so they can't recurse into unrelated sites.
func TestPostprocessItemDomainsCrawlHTMLAsset(t *testing.T) {
	previous := config.Get()
	config.Set(&config.Config{MaxHops: 0})
	domainscrawl.Reset()
	if err := domainscrawl.AddElements([]string{"example.com"}, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		domainscrawl.Reset()
		config.Set(previous)
	})

	for _, tc := range []struct {
		url        string
		wantAssets bool
	}{
		{"https://www.example.com/embed.html", true},
		{"https://other.org/original-article.html", false},
	} {
		seed := newArchivedHTMLItem(t, "https://www.example.com/article", "<html></html>")
		child := newArchivedHTMLItem(t, tc.url, `<html><body><img src="/photo.jpg"></body></html>`)
		if err := seed.AddChild(child, models.ItemGotChildren); err != nil {
			t.Fatal(err)
		}
		child.SetStatus(models.ItemArchived)

		postprocessItem(context.Background(), child)

		if got := child.HasChildren(); got != tc.wantAssets {
			t.Errorf("%s: extracted assets = %v, want %v", tc.url, got, tc.wantAssets)
		}
	}
}
