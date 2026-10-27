package extractor

import (
	"encoding/xml"
	"io"
	"strings"
	"time"

	"github.com/internetarchive/Zeno/v2/internal/pkg/log"
	"github.com/internetarchive/Zeno/v2/pkg/models"
)

var sitemapLogger = log.NewFieldedLogger(&log.Fields{
	"component": "postprocessor.extractor.sitemap",
})

// sitemapDateLayouts are the W3C datetime forms allowed in <lastmod> and
// <news:publication_date>, plus common non-conforming variants. Fractional
// seconds are accepted by every layout that has seconds.
var sitemapDateLayouts = []string{
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04Z07:00",
	"2006-01-02T15:04:05-0700",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02 15:04:05-0700",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"2006-01",
	"2006",
}

// sitemapEntry is a <url> of a <urlset> or a <sitemap> of a <sitemapindex>.
type sitemapEntry struct {
	loc   string
	date  time.Time // newest of <lastmod> and the publication dates, zero if none could be parsed
	media []string  // image:loc, video:content_loc, video:thumbnail_loc, xhtml:link alternates...
}

type sitemapDocument struct {
	root    string // local name of the root element, "urlset" or "sitemapindex" for a sitemap
	entries []sitemapEntry
}

func (d *sitemapDocument) isSitemap() bool {
	return d.root == "urlset" || d.root == "sitemapindex"
}

func (d *sitemapDocument) isIndex() bool {
	return d.root == "sitemapindex"
}

// parseSitemapDate parses a W3C datetime, a date without a time zone is UTC.
func parseSitemapDate(value string) (time.Time, bool) {
	for _, layout := range sitemapDateLayouts {
		if t, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func isHTTPURL(value string) bool {
	return len(value) >= 4 && strings.EqualFold(value[:4], "http")
}

// parseSitemap reads a sitemap or a sitemap index entry by entry. Elements are
// matched on their local name, so namespace prefixes (news:, image:, video:...)
// are ignored. On a parsing error, the entries read so far are returned.
func parseSitemap(r io.Reader) (*sitemapDocument, error) {
	decoder := xml.NewDecoder(r)
	decoder.Strict = false

	var (
		doc        = &sitemapDocument{}
		stack      []string // local names of the open elements
		entry      *sitemapEntry
		entryDepth int
		text       strings.Builder
	)

	addEntry := func() {
		if entry.loc != "" || len(entry.media) > 0 {
			doc.entries = append(doc.entries, *entry)
		}
		entry = nil
	}

	for {
		tok, err := decoder.RawToken()
		if err != nil {
			if entry != nil {
				addEntry()
			}
			if err == io.EOF {
				return doc, nil
			}
			return doc, err
		}

		switch tok := tok.(type) {
		case xml.StartElement:
			name := strings.ToLower(tok.Name.Local)
			stack = append(stack, name)
			text.Reset()

			if len(stack) == 1 {
				doc.root = name
				continue
			}

			if entry == nil {
				if len(stack) == 2 && (name == "url" || name == "sitemap") {
					entry = &sitemapEntry{}
					entryDepth = len(stack)
				}
				continue
			}

			// <xhtml:link rel="alternate" hreflang="..." href="..."/>
			if name == "link" {
				for _, attr := range tok.Attr {
					if strings.EqualFold(attr.Name.Local, "href") && isHTTPURL(attr.Value) {
						entry.media = append(entry.media, attr.Value)
					}
				}
			}
		case xml.CharData:
			if entry != nil {
				text.Write(tok)
			}
		case xml.EndElement:
			if len(stack) == 0 {
				continue
			}
			depth := len(stack)
			name := stack[depth-1]
			stack = stack[:depth-1]

			if entry == nil {
				continue
			}

			if depth == entryDepth {
				addEntry()
				continue
			}

			value := strings.TrimSpace(text.String())
			text.Reset()

			switch {
			case name == "loc" && depth == entryDepth+1:
				if isHTTPURL(value) {
					entry.loc = value
				}
			case name == "loc" || strings.HasSuffix(name, "_loc"):
				if isHTTPURL(value) {
					entry.media = append(entry.media, value)
				}
			case (name == "lastmod" && depth == entryDepth+1) || name == "publication_date":
				if date, ok := parseSitemapDate(value); ok && date.After(entry.date) {
					entry.date = date
				}
			}
		}
	}
}

// extractSitemapOutlinks returns the URLs listed in a sitemap or a sitemap
// index: every entry's <loc> and the media URLs attached to it. When maxAge is
// positive, entries dated before now-maxAge are dropped along with their media.
// Entries without a parseable date are always kept.
//
// Documents that IsSitemapXML matched but that are not a <urlset> or a
// <sitemapindex> are extracted like any other XML document.
func extractSitemapOutlinks(URL *models.URL, maxAge time.Duration, now time.Time) (outlinks []*models.URL, dropped int, err error) {
	defer URL.RewindBody()

	doc, err := parseSitemap(URL.GetBody())
	if !doc.isSitemap() {
		URL.RewindBody()
		assets, outlinks, err := XML(URL)
		return append(outlinks, assets...), 0, err
	}

	var cutoff time.Time
	if maxAge > 0 {
		cutoff = now.Add(-maxAge)
	}

	for _, entry := range doc.entries {
		if !cutoff.IsZero() && !entry.date.IsZero() && entry.date.Before(cutoff) {
			dropped++
			continue
		}

		if entry.loc != "" {
			outlinks = append(outlinks, &models.URL{Raw: entry.loc})
		}
		for _, media := range entry.media {
			outlinks = append(outlinks, &models.URL{Raw: media})
		}
	}

	sitemapLogger.Debug("extracted sitemap", "url", URL.Raw, "type", doc.root, "kept", len(doc.entries)-dropped, "dropped", dropped)

	return outlinks, dropped, err
}
