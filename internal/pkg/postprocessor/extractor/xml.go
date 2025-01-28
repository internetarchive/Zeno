package extractor

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"

	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/Zeno/pkg/models"
)

var sitemapMarker = []byte("sitemaps.org/schemas/sitemap/")

func IsXML(URL *models.URL) bool {
	return isContentType(URL.GetResponse().Header.Get("Content-Type"), "xml") && !IsSitemapXML(URL) && !URL.GetMIMEType().Is("image/svg+xml")
}

func IsSitemapXML(URL *models.URL) bool {
	defer URL.RewindBody()

	xmlBody, err := io.ReadAll(URL.GetBody())
	if err != nil {
		return false
	}

	return isContentType(URL.GetResponse().Header.Get("Content-Type"), "xml") && bytes.Contains(xmlBody, sitemapMarker)
}

func XML(URL *models.URL) (assets, outlinks []*models.URL, err error) {
	defer URL.RewindBody()

	xmlBody, err := io.ReadAll(URL.GetBody())
	if err != nil {
		return nil, nil, err
	}

	if len(xmlBody) == 0 {
		return nil, nil, errors.New("empty XML body")
	}

	decoder := xml.NewDecoder(bytes.NewReader(xmlBody))
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
				rawURLs = append(rawURLs, utils.DedupeStrings(LinkRegexRelaxed.FindAllString(string(tok), -1))...)
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
