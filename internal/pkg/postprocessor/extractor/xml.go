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

func XML(URL *models.URL) (assets []*models.URL, err error) {
	defer URL.RewindBody()

	xmlBody, err := io.ReadAll(URL.GetBody())
	if err != nil {
		return nil, err
	}

	if len(xmlBody) == 0 {
		return nil, errors.New("empty XML body")
	}

	decoder := xml.NewDecoder(bytes.NewReader(xmlBody))
	decoder.Strict = false

	var tok xml.Token
	var rawAssets []string
	for {
		tok, err = decoder.RawToken()

		if tok == nil && err == io.EOF {
			// normal EOF
			break
		}

		if err != nil {
			// return URLs we got so far when error occurs
			return assets, err
		}

		switch tok := tok.(type) {
		case xml.StartElement:
			for _, attr := range tok.Attr {
				if strings.HasPrefix(attr.Value, "http") {
					rawAssets = append(rawAssets, attr.Value)
				}
			}
		case xml.CharData:
			if bytes.HasPrefix(tok, []byte("http")) {
				rawAssets = append(rawAssets, string(tok))
			} else {
				// Try to extract URLs from the text
				rawAssets = append(rawAssets, utils.DedupeStrings(LinkRegexRelaxed.FindAllString(string(tok), -1))...)
			}
		}
	}

	for _, rawAsset := range rawAssets {
		assets = append(assets, &models.URL{
			Raw:  rawAsset,
			Hops: URL.GetHops() + 1,
		})
	}

	return assets, nil
}
