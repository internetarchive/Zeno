package extractor

import (
	"strings"

	"github.com/internetarchive/Zeno/pkg/models"
)

type LinkHeaderExtractor struct{}

func (l LinkHeaderExtractor) Support(m Mode) bool {
	return m == ModeGeneral
}

func (l LinkHeaderExtractor) Match(URL *models.URL) bool {
	return URL.GetResponse().Header.Get("link") != ""
}

// Extract parses a raw Link header in the form:
//
//	<url1>; rel="what", <url2>; rel="any"; another="yes", <url3>; rel="thing"
//
// returning a slice of models.URL structs
// Each of these are separated by a `, ` and the in turn by a `; `, with the first always being the url, and the remaining the key-val pairs
// See: https://simon-frey.com/blog/link-header/, https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Link
func (l LinkHeaderExtractor) Extract(URL *models.URL) (URLs []*models.URL, _ error) {
	var link = URL.GetResponse().Header.Get("link")

	if link == "" {
		return URLs, nil
	}

	for link := range strings.SplitSeq(link, ", ") {
		parts := strings.Split(link, ";")
		if len(parts) < 1 {
			// Malformed input, somehow we didn't get at least one part
			continue
		}

		extractedURL := strings.TrimSpace(strings.Trim(parts[0], "<>"))
		if extractedURL == "" {
			// Malformed input, URL is empty
			continue
		}

		for _, attrs := range parts[1:] {
			key, _ := parseAttr(attrs)
			if key == "" {
				// Malformed input, somehow the key is nothing
				continue
			}

			if key == "rel" {
				break
			}
		}

		URLs = append(URLs, &models.URL{
			Raw: extractedURL,
		})
	}

	return URLs, nil
}

// Parse a single attribute key value pair and return it
func parseAttr(attrs string) (key, value string) {
	kv := strings.SplitN(attrs, "=", 2)

	if len(kv) != 2 {
		return "", ""
	}

	key = strings.TrimSpace(kv[0])
	value = strings.TrimSpace(strings.Trim(kv[1], "\""))

	return key, value
}
