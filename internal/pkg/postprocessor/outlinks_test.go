package postprocessor

import (
	"context"
	_ "embed"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/internetarchive/Zeno/v2/internal/pkg/config"
	"github.com/internetarchive/Zeno/v2/internal/pkg/utils"
	"github.com/internetarchive/Zeno/v2/pkg/models"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

func TestFilterURLsByProtocol(t *testing.T) {
	var outlinks []*models.URL
	outlinks = append(outlinks, &models.URL{Raw: "http://example.com"})
	// skipped
	outlinks = append(outlinks, &models.URL{Raw: "tel:12312313"})
	outlinks = append(outlinks, &models.URL{Raw: "MAILTO:someone@archive.org"})
	outlinks = append(outlinks, &models.URL{Raw: "file:/tmp/data.dat"})

	filtered := filterURLsByProtocol(outlinks)

	if len(filtered) != 1 {
		t.Errorf("expected 1 filtered, got %d", len(filtered))
	}
}

func TestFilterMaxOutlinks(t *testing.T) {
	var outlinks []*models.URL
	outlinks = append(outlinks, &models.URL{Raw: "http://e1.com"})
	outlinks = append(outlinks, &models.URL{Raw: "http://e2.com"})
	outlinks = append(outlinks, &models.URL{Raw: "http://e3.com"})

	config.InitConfig()

	// no limit by default
	outlinks2 := filterMaxOutlinks(outlinks)
	if len(outlinks2) != 3 {
		t.Errorf("expected 3 outlinks and no filtering, got %d", len(outlinks2))
	}

	// set limit = 1
	config.Get().MaxOutlinks = 1
	outlinks3 := filterMaxOutlinks(outlinks)
	if len(outlinks3) != 1 {
		t.Errorf("expected 1 outlink, got %d", len(outlinks3))
	}
}

//go:embed testdata/wikipedia_IA.txt
var wikitext []byte // CC BY-SA 4.0

//go:embed testdata/Q27536592.html.gz
var q27536592HTMLGZ []byte // CC BY-SA 4.0

func TestExtractLinksFromPage(t *testing.T) {
	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write(wikitext)
	URL := &models.URL{Raw: "https://en.wikipedia.org/wiki/Internet_Archive"}
	URL.SetBody(spooledTempFile)
	URL.Parse()

	config.InitConfig()

	config.Get().StrictRegex = false
	links := extractLinksFromPage(URL)
	if len(links) != 430 {
		t.Errorf("expected 430 links, got %d", len(links))
	}
	config.Get().StrictRegex = true
	links = extractLinksFromPage(URL)
	if len(links) != 433 {
		t.Errorf("expected 433 links, got %d", len(links))
	}
}

// https://github.com/internetarchive/Zeno/issues/413
//
// There are 2 lines in the HTML that are longer than 64KiB, overflowing the default bufio.Scanner buffer size if we use line-by-line reading.
func TestExtractLinksFromPageWithLongLines(t *testing.T) {
	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write(utils.MustDecompressGzippedBytes(q27536592HTMLGZ))

	URL := &models.URL{Raw: "https://www.wikidata.org/wiki/Q27536592"}
	URL.SetBody(spooledTempFile)
	URL.Parse()

	config.InitConfig()
	config.Get().StrictRegex = false

	links := extractLinksFromPage(URL)
	if len(links) != 72 {
		t.Errorf("expected 72 links, got %d", len(links))
	}
}

func newSitemapTestItem(t *testing.T, rawURL, body, contentType string, hops int) *models.Item {
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
	resp.Header.Set("Content-Type", contentType)
	newURL.SetResponse(resp)

	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write([]byte(body))
	t.Cleanup(func() { spooledTempFile.Close() })
	newURL.SetBody(spooledTempFile)
	newURL.SetHops(hops)

	return models.NewItem(&newURL, "")
}

func setTestConfig(t *testing.T, cfg *config.Config) {
	t.Helper()

	previous := config.Get()
	config.Set(cfg)
	t.Cleanup(func() { config.Set(previous) })
}

func TestExtractOutlinksSitemapServedAsText(t *testing.T) {
	setTestConfig(t, &config.Config{})

	sitemap := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
	xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
	xsi:schemaLocation="http://www.sitemaps.org/schemas/sitemap/0.9 http://www.sitemaps.org/schemas/sitemap/0.9/sitemap.xsd">
	<url><loc>https://example.com/article</loc></url>
</urlset>`

	for _, contentType := range []string{"text/xml", "application/xml"} {
		item := newSitemapTestItem(t, "https://example.com/sitemap.xml", sitemap, contentType, 0)

		outlinks, err := extractOutlinks(context.Background(), item)
		if err != nil {
			t.Fatalf("extractOutlinks() failed: %v", err)
		}

		if len(outlinks) != 1 || outlinks[0].Raw != "https://example.com/article" {
			t.Errorf("%s: expected only https://example.com/article, got %v", contentType, rawOutlinks(outlinks))
		}
		for _, outlink := range outlinks {
			if outlink.GetHops() != 1 {
				t.Errorf("%s: expected %s at hop 1, got %d", contentType, outlink.Raw, outlink.GetHops())
			}
		}
	}
}

func TestExtractOutlinksSitemapIndexHops(t *testing.T) {
	yoastIndex := `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<sitemap><loc>https://example.com/post-sitemap.xml</loc><lastmod>2026-10-26T10:00:00+00:00</lastmod></sitemap>
	<sitemap><loc>https://example.com/page-sitemap.xml</loc></sitemap>
</sitemapindex>`

	crossHostIndex := `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<sitemap><loc>https://www.example.com/news-sitemap.xml</loc></sitemap>
	<sitemap><loc>https://cdn.example.net/sitemap.xml</loc></sitemap>
	<sitemap><loc>https://example.com.evil.org/sitemap.xml</loc></sitemap>
</sitemapindex>`

	urlset := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>https://example.com/2026/10/26/article/</loc></url>
	<url><loc>https://example.com/nested-sitemap.xml</loc></url>
</urlset>`

	tests := []struct {
		name        string
		url         string
		body        string
		hops        int
		sameHop     bool
		expectedHop map[string]int
	}{
		{
			name:    "Yoast index, flag on, children keep the index's hop",
			url:     "https://example.com/sitemap_index.xml",
			body:    yoastIndex,
			hops:    0,
			sameHop: true,
			expectedHop: map[string]int{
				"https://example.com/post-sitemap.xml": 0,
				"https://example.com/page-sitemap.xml": 0,
			},
		},
		{
			name:    "Nested index at hop 2, flag on",
			url:     "https://example.com/sitemap_index.xml",
			body:    yoastIndex,
			hops:    2,
			sameHop: true,
			expectedHop: map[string]int{
				"https://example.com/post-sitemap.xml": 2,
				"https://example.com/page-sitemap.xml": 2,
			},
		},
		{
			name:    "Yoast index, flag off, regression",
			url:     "https://example.com/sitemap_index.xml",
			body:    yoastIndex,
			hops:    0,
			sameHop: false,
			expectedHop: map[string]int{
				"https://example.com/post-sitemap.xml": 1,
				"https://example.com/page-sitemap.xml": 1,
			},
		},
		{
			name:    "Index on www host, children on other hosts get hop+1",
			url:     "https://www.example.com/sitemap_index.xml",
			body:    crossHostIndex,
			hops:    0,
			sameHop: true,
			expectedHop: map[string]int{
				"https://www.example.com/news-sitemap.xml": 0,
				"https://cdn.example.net/sitemap.xml":      1,
				"https://example.com.evil.org/sitemap.xml": 1,
			},
		},
		{
			name:    "Index without www, child with www keeps the hop",
			url:     "https://EXAMPLE.com/sitemap_index.xml",
			body:    crossHostIndex,
			hops:    1,
			sameHop: true,
			expectedHop: map[string]int{
				"https://www.example.com/news-sitemap.xml": 1,
				"https://cdn.example.net/sitemap.xml":      2,
				"https://example.com.evil.org/sitemap.xml": 2,
			},
		},
		{
			name:    "Urlset, flag on, entries get hop+1",
			url:     "https://example.com/post-sitemap.xml",
			body:    urlset,
			hops:    0,
			sameHop: true,
			expectedHop: map[string]int{
				"https://example.com/2026/10/26/article/": 1,
				"https://example.com/nested-sitemap.xml":  1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setTestConfig(t, &config.Config{SitemapIndexSameHop: tt.sameHop})

			item := newSitemapTestItem(t, tt.url, tt.body, "application/xml", tt.hops)

			outlinks, err := extractOutlinks(context.Background(), item)
			if err != nil {
				t.Fatalf("extractOutlinks() failed: %v", err)
			}

			if len(outlinks) != len(tt.expectedHop) {
				t.Fatalf("expected %d outlinks, got %v", len(tt.expectedHop), rawOutlinks(outlinks))
			}
			for _, outlink := range outlinks {
				expected, ok := tt.expectedHop[outlink.Raw]
				if !ok {
					t.Errorf("unexpected outlink %s", outlink.Raw)
					continue
				}
				if outlink.GetHops() != expected {
					t.Errorf("%s: expected hop %d, got %d", outlink.Raw, expected, outlink.GetHops())
				}
			}
		})
	}
}

func rawOutlinks(outlinks []*models.URL) []string {
	var raws []string
	for _, outlink := range outlinks {
		raws = append(raws, outlink.Raw)
	}
	return raws
}

func BenchmarkExtractLinksFromPageRelax(b *testing.B) {
	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write(wikitext)
	URL := &models.URL{Raw: "https://en.wikipedia.org/wiki/Internet_Archive"}
	URL.SetBody(spooledTempFile)
	URL.Parse()

	config.InitConfig()
	config.Get().StrictRegex = false

	for b.Loop() {
		extractLinksFromPage(URL)
	}

	// add metric: KiB/s
	totalSize := len(wikitext) * b.N
	kiB := float64(totalSize) / 1024.0
	seconds := b.Elapsed().Seconds()
	kiBPerSecond := kiB / seconds
	b.ReportMetric(kiBPerSecond, "KiB/s")
}

func BenchmarkExtractLinksFromPageStrict(b *testing.B) {
	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	spooledTempFile.Write(wikitext)
	URL := &models.URL{Raw: "https://en.wikipedia.org/wiki/Internet_Archive"}
	URL.SetBody(spooledTempFile)
	URL.Parse()

	config.InitConfig()
	config.Get().StrictRegex = true

	for b.Loop() {
		extractLinksFromPage(URL)
	}

	// add metric: KiB/s
	totalSize := len(wikitext) * b.N
	kiB := float64(totalSize) / 1024.0
	seconds := b.Elapsed().Seconds()
	kiBPerSecond := kiB / seconds
	b.ReportMetric(kiBPerSecond, "KiB/s")
}
