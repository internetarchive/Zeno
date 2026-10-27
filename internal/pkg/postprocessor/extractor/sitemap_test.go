package extractor

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/internetarchive/Zeno/v2/pkg/models"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
)

// sitemapTestNow is the fixed "now" used by the sitemap tests.
var sitemapTestNow = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func newSitemapTestURL(t *testing.T, body string) *models.URL {
	t.Helper()

	spooledTempFile := spooledtempfile.NewSpooledTempFile("test", os.TempDir(), 2048, false, -1)
	if _, err := spooledTempFile.Write([]byte(body)); err != nil {
		t.Fatalf("failed to write body: %v", err)
	}
	t.Cleanup(func() { spooledTempFile.Close() })

	URL := &models.URL{Raw: "https://example.com/sitemap.xml"}
	URL.SetBody(spooledTempFile)
	return URL
}

func rawURLs(URLs []*models.URL) []string {
	var raws []string
	for _, URL := range URLs {
		raws = append(raws, URL.Raw)
	}
	return raws
}

const sitemapIndexMixedLastmod = `<?xml version="1.0" encoding="UTF-8"?>
<?xml-stylesheet type="text/xsl" href="https://example.com/wp-content/plugins/wordpress-seo/css/main-sitemap.xsl"?>
<sitemapindex xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
	xsi:schemaLocation="http://www.sitemaps.org/schemas/sitemap/0.9 https://www.sitemaps.org/schemas/sitemap/0.9/siteindex.xsd"
	xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<!-- See https://developers.google.com/search/docs/crawling-indexing/sitemaps/overview -->
	<sitemap>
		<loc>https://example.com/post-sitemap1.xml</loc>
		<lastmod>2019-03-01T10:00:00+00:00</lastmod>
	</sitemap>
	<sitemap>
		<loc>https://example.com/post-sitemap2.xml</loc>
		<lastmod>2026-10-26T23:15:00+00:00</lastmod>
	</sitemap>
	<sitemap>
		<loc>https://example.com/page-sitemap.xml</loc>
	</sitemap>
	<sitemap>
		<loc>https://example.com/category-sitemap.xml</loc>
		<lastmod>2026-10-27</lastmod>
	</sitemap>
</sitemapindex>`

const urlsetMixedLastmod = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
	xmlns:image="http://www.google.com/schemas/sitemap-image/1.1"
	xmlns:xhtml="http://www.w3.org/1999/xhtml">
	<url>
		<loc>https://example.com/recent</loc>
		<lastmod>2026-10-27T08:00:00Z</lastmod>
		<image:image><image:loc>https://example.com/recent.jpg</image:loc></image:image>
		<xhtml:link rel="alternate" hreflang="es" href="https://example.com/es/recent"/>
	</url>
	<url>
		<loc>https://example.com/old</loc>
		<lastmod>2021-05-04T08:00:00.123456+02:00</lastmod>
		<image:image><image:loc>https://example.com/old-1.jpg</image:loc></image:image>
		<image:image><image:loc>https://example.com/old-2.jpg</image:loc></image:image>
		<xhtml:link rel="alternate" hreflang="es" href="https://example.com/es/old"/>
	</url>
	<url>
		<loc>https://example.com/no-lastmod</loc>
	</url>
	<url>
		<loc>https://example.com/bad-lastmod</loc>
		<lastmod>last tuesday</lastmod>
	</url>
	<url>
		<loc>https://example.com/future</loc>
		<lastmod>2027-01-01</lastmod>
	</url>
	<url>
		<loc> https://example.com/whitespace </loc>
		<lastmod> 2026-10-26 </lastmod>
	</url>
</urlset>`

const googleNewsSitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
	xmlns:news="http://www.google.com/schemas/sitemap-news/0.9">
	<url>
		<loc>https://example.com/news/fresh</loc>
		<news:news>
			<news:publication>
				<news:name>Example Times</news:name>
				<news:language>en</news:language>
			</news:publication>
			<news:publication_date>2026-10-27T06:30:00-04:00</news:publication_date>
			<news:title>Fresh story</news:title>
		</news:news>
	</url>
	<url>
		<loc>https://example.com/news/stale</loc>
		<news:news>
			<news:publication_date>2026-10-01T06:30:00-04:00</news:publication_date>
			<news:title>Stale story</news:title>
		</news:news>
	</url>
	<url>
		<loc>https://example.com/news/republished</loc>
		<lastmod>2020-01-01</lastmod>
		<news:news>
			<news:publication_date>2026-10-27T01:00:00Z</news:publication_date>
		</news:news>
	</url>
</urlset>`

const videoSitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
	xmlns:video="http://www.google.com/schemas/sitemap-video/1.1">
	<url>
		<loc>https://example.com/videos/new</loc>
		<video:video>
			<video:thumbnail_loc>https://example.com/thumbs/new.jpg</video:thumbnail_loc>
			<video:title>New</video:title>
			<video:content_loc>https://example.com/video/new.mp4</video:content_loc>
			<video:publication_date>2026-10-26T12:00:00+00:00</video:publication_date>
		</video:video>
	</url>
	<url>
		<loc>https://example.com/videos/old</loc>
		<video:video>
			<video:thumbnail_loc>https://example.com/thumbs/old.jpg</video:thumbnail_loc>
			<video:content_loc>https://example.com/video/old.mp4</video:content_loc>
			<video:publication_date>2015-10-26T12:00:00+00:00</video:publication_date>
		</video:video>
	</url>
</urlset>`

func TestExtractSitemapOutlinks(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		maxAge      time.Duration
		expected    []string
		dropped     int
		expectError bool
	}{
		{
			name:   "Sitemap index, only recent children are returned",
			body:   sitemapIndexMixedLastmod,
			maxAge: 72 * time.Hour,
			expected: []string{
				"https://example.com/post-sitemap2.xml",
				"https://example.com/page-sitemap.xml",
				"https://example.com/category-sitemap.xml",
			},
			dropped: 1,
		},
		{
			name:   "Sitemap index, no filtering",
			body:   sitemapIndexMixedLastmod,
			maxAge: 0,
			expected: []string{
				"https://example.com/post-sitemap1.xml",
				"https://example.com/post-sitemap2.xml",
				"https://example.com/page-sitemap.xml",
				"https://example.com/category-sitemap.xml",
			},
		},
		{
			name:   "Urlset, recent, missing, unparseable and future lastmods are kept, old ones are dropped with their media",
			body:   urlsetMixedLastmod,
			maxAge: 72 * time.Hour,
			expected: []string{
				"https://example.com/recent",
				"https://example.com/recent.jpg",
				"https://example.com/es/recent",
				"https://example.com/no-lastmod",
				"https://example.com/bad-lastmod",
				"https://example.com/future",
				"https://example.com/whitespace",
			},
			dropped: 1,
		},
		{
			name:   "Urlset, no filtering",
			body:   urlsetMixedLastmod,
			maxAge: 0,
			expected: []string{
				"https://example.com/recent",
				"https://example.com/recent.jpg",
				"https://example.com/es/recent",
				"https://example.com/old",
				"https://example.com/old-1.jpg",
				"https://example.com/old-2.jpg",
				"https://example.com/es/old",
				"https://example.com/no-lastmod",
				"https://example.com/bad-lastmod",
				"https://example.com/future",
				"https://example.com/whitespace",
			},
		},
		{
			name:   "Google News sitemap, publication_date without lastmod, newest date wins",
			body:   googleNewsSitemap,
			maxAge: 48 * time.Hour,
			expected: []string{
				"https://example.com/news/fresh",
				"https://example.com/news/republished",
			},
			dropped: 1,
		},
		{
			name:   "Video sitemap, media of old entries are dropped",
			body:   videoSitemap,
			maxAge: 72 * time.Hour,
			expected: []string{
				"https://example.com/videos/new",
				"https://example.com/thumbs/new.jpg",
				"https://example.com/video/new.mp4",
			},
			dropped: 1,
		},
		{
			name: "Prefixed sitemap elements",
			body: `<?xml version="1.0" encoding="UTF-8"?>
<sm:urlset xmlns:sm="http://www.sitemaps.org/schemas/sitemap/0.9">
	<sm:url><sm:loc>https://example.com/a</sm:loc><sm:lastmod>2026-10-27</sm:lastmod></sm:url>
	<sm:url><sm:loc>https://example.com/b</sm:loc><sm:lastmod>2001-10-27</sm:lastmod></sm:url>
</sm:urlset>`,
			maxAge:   24 * time.Hour,
			expected: []string{"https://example.com/a"},
			dropped:  1,
		},
		{
			name: "CDATA and entities in loc",
			body: `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc><![CDATA[https://example.com/?a=1&b=2]]></loc></url>
	<url><loc>https://example.com/?c=3&amp;d=4</loc></url>
	<url><loc>/relative/is/not/a/sitemap/url</loc></url>
</urlset>`,
			expected: []string{
				"https://example.com/?a=1&b=2",
				"https://example.com/?c=3&d=4",
			},
		},
		{
			name: "Truncated sitemap returns the entries read so far",
			body: `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>https://example.com/a</loc></url>
	<url><loc>https://example.com/b</loc><lastm`,
			expected: []string{
				"https://example.com/a",
				"https://example.com/b",
			},
			expectError: true,
		},
		{
			name: "Document mentioning the sitemap schema that isn't a sitemap falls back to generic XML extraction",
			body: `<?xml version="1.0" encoding="UTF-8"?>
<!-- http://www.sitemaps.org/schemas/sitemap/0.9 -->
<rss version="2.0">
	<channel>
		<link>https://example.com/feed-link</link>
	</channel>
</rss>`,
			maxAge: time.Hour,
			expected: []string{
				"https://example.com/feed-link",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			URL := newSitemapTestURL(t, tt.body)

			outlinks, dropped, err := extractSitemapOutlinks(URL, tt.maxAge, sitemapTestNow)
			if (err != nil) != tt.expectError {
				t.Fatalf("extractSitemapOutlinks() error = %v, expectError %v", err, tt.expectError)
			}

			got := rawURLs(outlinks)
			if !slices.Equal(got, tt.expected) {
				t.Errorf("unexpected outlinks\n got: %v\nwant: %v", got, tt.expected)
			}

			if dropped != tt.dropped {
				t.Errorf("expected %d dropped entries, got %d", tt.dropped, dropped)
			}

			for _, outlink := range got {
				if strings.Contains(outlink, "sitemaps.org") || strings.Contains(outlink, "w3.org") ||
					strings.Contains(outlink, "google.com/schemas") || strings.Contains(outlink, "developers.google.com") {
					t.Errorf("namespace or schema URL returned: %s", outlink)
				}
			}
		})
	}
}

// With filtering disabled, every <loc> found by the generic XML extractor is
// still returned. The only URLs lost are namespace, schema and comment URLs.
func TestExtractSitemapOutlinksNoFilteringRegression(t *testing.T) {
	for _, body := range []string{sitemapIndexMixedLastmod, urlsetMixedLastmod, googleNewsSitemap, videoSitemap} {
		URL := newSitemapTestURL(t, body)

		assets, xmlOutlinks, err := XML(URL)
		if err != nil {
			t.Fatalf("XML() failed: %v", err)
		}
		before := rawURLs(append(xmlOutlinks, assets...))

		outlinks, dropped, err := extractSitemapOutlinks(URL, 0, sitemapTestNow)
		if err != nil {
			t.Fatalf("extractSitemapOutlinks() failed: %v", err)
		}
		if dropped != 0 {
			t.Errorf("expected no dropped entries without --sitemap-max-age, got %d", dropped)
		}
		after := rawURLs(outlinks)

		for _, raw := range before {
			isJunk := strings.Contains(raw, "sitemaps.org") || strings.Contains(raw, "w3.org") ||
				strings.Contains(raw, "google.com/schemas") || strings.Contains(raw, "developers.google.com")
			if isJunk {
				continue
			}
			if !slices.Contains(after, strings.TrimSpace(raw)) {
				t.Errorf("URL %q extracted by XML() is missing", raw)
			}
		}
	}
}

func TestExtractSitemapOutlinksMarksIndexChildren(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		expectedChild bool
	}{
		{"Sitemap index children are marked", sitemapIndexMixedLastmod, true},
		{"Urlset entries and media are not marked", urlsetMixedLastmod, false},
		{"Google News entries are not marked", googleNewsSitemap, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outlinks, _, err := extractSitemapOutlinks(newSitemapTestURL(t, tt.body), 0, sitemapTestNow)
			if err != nil {
				t.Fatalf("extractSitemapOutlinks() failed: %v", err)
			}
			if len(outlinks) == 0 {
				t.Fatal("expected outlinks")
			}
			for _, outlink := range outlinks {
				if outlink.IsSitemapIndexChild() != tt.expectedChild {
					t.Errorf("%s: expected IsSitemapIndexChild() = %v", outlink.Raw, tt.expectedChild)
				}
			}
		})
	}
}

func TestParseSitemapRoot(t *testing.T) {
	tests := []struct {
		body    string
		root    string
		isIndex bool
	}{
		{sitemapIndexMixedLastmod, "sitemapindex", true},
		{urlsetMixedLastmod, "urlset", false},
		{`<?xml version="1.0"?><rss><channel></channel></rss>`, "rss", false},
	}

	for _, tt := range tests {
		doc, err := parseSitemap(strings.NewReader(tt.body))
		if err != nil {
			t.Fatalf("parseSitemap() failed: %v", err)
		}
		if doc.root != tt.root {
			t.Errorf("expected root %q, got %q", tt.root, doc.root)
		}
		if doc.isIndex() != tt.isIndex {
			t.Errorf("root %q: expected isIndex() = %v", doc.root, tt.isIndex)
		}
	}
}

func TestParseSitemapDate(t *testing.T) {
	tests := []struct {
		value    string
		expected time.Time
		ok       bool
	}{
		{"2026-10-27", time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC), true},
		{"2026-10", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), true},
		{"2026", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), true},
		{"2026-10-27T15:04Z", time.Date(2026, 10, 27, 15, 4, 0, 0, time.UTC), true},
		{"2026-10-27T15:04+02:00", time.Date(2026, 10, 27, 13, 4, 0, 0, time.UTC), true},
		{"2026-10-27T15:04:05Z", time.Date(2026, 10, 27, 15, 4, 5, 0, time.UTC), true},
		{"2026-10-27T15:04:05-05:00", time.Date(2026, 10, 27, 20, 4, 5, 0, time.UTC), true},
		{"2026-10-27T15:04:05.123456789+00:00", time.Date(2026, 10, 27, 15, 4, 5, 123456789, time.UTC), true},
		{"2026-10-27T15:04:05", time.Date(2026, 10, 27, 15, 4, 5, 0, time.UTC), true},
		{"2026-10-27T15:04:05+0100", time.Date(2026, 10, 27, 14, 4, 5, 0, time.UTC), true},
		{"2026-10-27 15:04:05", time.Date(2026, 10, 27, 15, 4, 5, 0, time.UTC), true},
		{"", time.Time{}, false},
		{"yesterday", time.Time{}, false},
		{"27/10/2026", time.Time{}, false},
		{"2026-13-45", time.Time{}, false},
	}

	for _, tt := range tests {
		got, ok := parseSitemapDate(tt.value)
		if ok != tt.ok {
			t.Errorf("parseSitemapDate(%q): expected ok=%v, got %v", tt.value, tt.ok, ok)
			continue
		}
		if ok && !got.Equal(tt.expected) {
			t.Errorf("parseSitemapDate(%q): expected %v, got %v", tt.value, tt.expected, got)
		}
	}
}
