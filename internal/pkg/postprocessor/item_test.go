package postprocessor

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/internetarchive/Zeno/v2/internal/pkg/config"
	"github.com/internetarchive/Zeno/v2/internal/pkg/postprocessor/domainscrawl"
	"github.com/internetarchive/Zeno/v2/pkg/models"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

// newArchivedHTMLURL returns a URL that looks like an archived HTML page with the given body
func newArchivedHTMLURL(t *testing.T, rawURL, contentType, html string) *models.URL {
	t.Helper()

	resp := &http.Response{
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(html)),
		StatusCode: 200,
	}
	resp.Header.Set("Content-Type", contentType)

	newURL, err := models.NewURL(rawURL)
	if err != nil {
		t.Fatalf("error parsing URL: %v", err)
	}
	newURL.SetResponse(resp)

	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write([]byte(html))
	newURL.SetBody(spooledTempFile)

	return &newURL
}

// A page that references itself (here with <link href="#">) must not get itself as an asset,
// whatever the order of its query parameters. On a non-UTF-8 page the query used to be
// rebuilt in a different order, so the page was captured as its own asset in an endless loop.
func TestExtractAssets_SelfReferenceWithQuery(t *testing.T) {
	config.Set(&config.Config{})
	config.Get().DisableHTMLTag = []string{}

	html := `<html><head><link rel="schema.DC" href="#" /></head><body><img src="img.png"></body></html>`
	rawURL := "http://example.com/doc.html?ZyActionD=ZyDocument&Client=EPA&Query=600R05032&ZyEntry=1&SeekPage=b&User=ANONYMOUS"

	for range 20 {
		newURL := newArchivedHTMLURL(t, rawURL, "text/html; charset=windows-1252", html)
		item := models.NewItem(newURL, "")

		assets, _, err := ExtractAssetsOutlinks(item)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if len(assets) != 1 || assets[0].Raw != "http://example.com/img.png" {
			var raws []string
			for _, asset := range assets {
				raws = append(raws, asset.Raw)
			}
			t.Fatalf("expected only http://example.com/img.png as asset, got %v", raws)
		}
	}
}

// In a domains crawl, items deeper than 2 still give outlinks but their assets are not captured.
func TestPostprocessItem_DomainsCrawlAssetDepth(t *testing.T) {
	config.Set(&config.Config{})
	config.Get().DisableHTMLTag = []string{}

	config.Get().MaxCSSJump = 1

	domainscrawl.Reset()
	if err := domainscrawl.AddElements([]string{"example.com"}, nil); err != nil {
		t.Fatalf("unable to add domains crawl elements: %v", err)
	}
	defer domainscrawl.Reset()

	html := `<html><head><link href="style.css"><style>@import url("imported.css");</style></head><body><a href="http://example.com/next">next</a></body></html>`

	tests := []struct {
		name         string
		depth        int
		wantChildren bool
	}{
		{name: "depth 2 captures assets", depth: 2, wantChildren: true},
		{name: "depth 3 does not capture assets", depth: 3, wantChildren: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := models.NewItem(newArchivedHTMLURL(t, "http://example.com/0", "text/html", html), "")
			for i := 1; i <= tt.depth; i++ {
				child := models.NewItem(newArchivedHTMLURL(t, "http://example.com/"+string(rune('0'+i)), "text/html", html), "")
				if err := item.AddChild(child, models.ItemGotChildren); err != nil {
					t.Fatalf("unable to add child: %v", err)
				}
				item = child
			}
			item.SetStatus(models.ItemArchived)

			outlinks := postprocessItem(context.Background(), item)

			if item.HasChildren() != tt.wantChildren {
				t.Errorf("expected children: %v, got children: %v", tt.wantChildren, item.HasChildren())
			}
			if len(outlinks) == 0 {
				t.Fatalf("expected http://example.com/next as outlink, got none")
			}
			for _, outlink := range outlinks {
				if outlink.GetURL().Raw != "http://example.com/next" {
					t.Errorf("expected only http://example.com/next as outlink, got %s", outlink.GetURL().Raw)
				}
			}
		})
	}
}
