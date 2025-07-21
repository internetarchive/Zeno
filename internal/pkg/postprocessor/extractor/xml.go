package extractor

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"

	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/Zeno/pkg/models"
)

var sitemapMarker = []byte("sitemaps.org/schemas/sitemap/")

// check if the Content-Type or MIME-type indicates XML
// exclude sitemap and SVG
func IsXML(URL *models.URL) bool {
	return URL.GetMIMEType() != nil &&
		strings.Contains(URL.GetMIMEType().String(), "xml") &&
		!IsSitemapXML(URL) && !URL.GetMIMEType().Is("image/svg+xml")
}

func IsSitemapXML(URL *models.URL) bool {
	defer URL.RewindBody()

	decoder := xml.NewDecoder(URL.GetBody())
	decoder.Strict = false

	for {
		tok, err := decoder.RawToken()
		if err == io.EOF {
			// We've read the entire XML, no match found
			break
		}
		if err != nil {
			// If there's any parsing error, we consider it not a sitemap
			return false
		}

		switch t := tok.(type) {

		// --- TEXT-LIKE tokens ---
		case xml.CharData:
			// Normal text content
			if bytes.Contains(t, sitemapMarker) {
				return true
			}
		case xml.Comment:
			// <!-- comment content -->
			if bytes.Contains(t, sitemapMarker) {
				return true
			}
		case xml.Directive:
			// <!DOCTYPE or <!ENTITY ...>
			if bytes.Contains(t, sitemapMarker) {
				return true
			}
		case xml.ProcInst:
			// <?xml-stylesheet ...?>
			// t.Target is string, t.Inst is []byte
			if bytes.Contains(t.Inst, sitemapMarker) {
				return true
			}

		// --- ELEMENT tokens ---
		case xml.StartElement:
			// 1) Check element's namespace or local name
			//    e.g. <urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
			//    t.Name.Space could be "http://www.sitemaps.org/schemas/sitemap/0.9"
			//    t.Name.Local might be "urlset"
			//
			//    But in practice, many sitemap docs have the namespace in the default XMLNS,
			//    so we should also check attributes.
			if strings.Contains(t.Name.Space, string(sitemapMarker)) {
				return true
			}
			if strings.Contains(t.Name.Local, string(sitemapMarker)) {
				return true
			}

			// 2) Check attributes (common place for the sitemap XMLNS)
			for _, attr := range t.Attr {
				if strings.Contains(attr.Value, string(sitemapMarker)) {
					return true
				}
			}

		case xml.EndElement:
			// EndElement typically has no textual data, so nothing to check
			continue
		}
	}
	return false
}

func XML(URL *models.URL) (assets, outlinks []*models.URL, err error) {
	defer URL.RewindBody()

	body := bufio.NewReader(URL.GetBody())

	// Peek to check if body has any non-whitespace content
	peek, err := body.Peek(512) // peek up to 512 bytes
	if err != nil && err != io.EOF {
		return nil, nil, err
	}
	if len(bytes.TrimSpace(peek)) == 0 {
		return nil, nil, errors.New("empty XML body")
	}

	decoder := xml.NewDecoder(body)
	decoder.Strict = false

	var tok xml.Token
	var rawURLs []string
	for {
		tok, err = decoder.RawToken()

		if tok == nil && err == io.EOF {
			// normal EOF
			break
		}

		if err != nil {
			// return URLs we got so far when error occurs
			return assets, outlinks, err
		}

		switch tok := tok.(type) {
		case xml.StartElement:
			for _, attr := range tok.Attr {
				if strings.HasPrefix(attr.Value, "http") {
					rawURLs = append(rawURLs, attr.Value)
				}
			}
		case xml.CharData:
			if bytes.HasPrefix(tok, []byte("http")) {
				rawURLs = append(rawURLs, string(tok))
			} else {
				// Try to extract URLs from the text
				rawURLs = append(rawURLs, utils.DedupeStrings(LinkRegexStrict.FindAllString(string(tok), -1))...)
			}
		}
	}

	// We only consider as assets the URLs in which we can find a file extension
	for _, rawURL := range rawURLs {
		if hasFileExtension(rawURL) {
			assets = append(assets, &models.URL{
				Raw: rawURL,
			})
		} else {
			outlinks = append(outlinks, &models.URL{
				Raw: rawURL,
			})
		}
	}

	return assets, outlinks, nil
}

type SitemapXMLOutlinkExtractor struct{}

func (SitemapXMLOutlinkExtractor) Match(URL *models.URL) bool {
	return IsSitemapXML(URL)
}

func (SitemapXMLOutlinkExtractor) Extract(URL *models.URL) ([]*models.URL, error) {
	assets, outlinks, err := XML(URL)
	if err != nil {
		return outlinks, err
	}

	// Here we don't care about the difference between assets and outlinks,
	// we just want to extract all the URLs from the sitemap
	outlinks = append(outlinks, assets...)
	return outlinks, nil
}
