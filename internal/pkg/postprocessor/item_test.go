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
