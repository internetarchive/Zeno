package extractor

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func XML(resp *http.Response) (URLs []*url.URL, sitemap bool, err error) {
	xmlBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, sitemap, err
	}

	if strings.Contains(string(xmlBody), "sitemaps.org/schemas/sitemap/") {
		sitemap = true
	}

	reader := bytes.NewReader(xmlBody)
	decoder := xml.NewDecoder(reader)

	// try to decode one token to see if stream is open
	_, err = decoder.Token()
	if err != nil {
		return nil, sitemap, err
	}

	// seek back to 0 if we are still here
	reader.Seek(0, 0)
	decoder = xml.NewDecoder(reader)

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, sitemap, err
		}

		switch tok := tok.(type) {
		case xml.StartElement:
			for _, attr := range tok.Attr {
				if strings.HasPrefix(attr.Value, "http") {
					parsedURL, err := url.Parse(attr.Value)
					if err == nil {
						URLs = append(URLs, parsedURL)
					}
				}
			}
		case xml.CharData:
			if strings.HasPrefix(string(tok), "http") {
				parsedURL, err := url.Parse(string(tok))
				if err == nil {
					URLs = append(URLs, parsedURL)
				}
			}
		}
	}

	return URLs, sitemap, nil
}
