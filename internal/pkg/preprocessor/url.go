package preprocessor

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/internetarchive/Zeno/pkg/models"
	wu "github.com/nlnwa/whatwg-url/url"
)

const (
	httpPrefix  = "http://"
	httpsPrefix = "https://"
	ftpPrefix   = "ftp://"
)

// Normalize the URL by removing fragments, attempting to add URL scheme if missing,
// and converting relative URLs into absolute URLs. An error is returned if the URL
// cannot be normalized.
func NormalizeURL(URL *models.URL, parentURL *models.URL) (err error) {
	// Clean the URL by removing leading and trailing quotes
	URL.Raw = strings.Trim(URL.Raw, `"'`)

	var wuParse *wu.Url

	if parentURL == nil {
		wuParse, err = wu.Parse(URL.Raw)
		if err != nil {
			lowerURL := strings.ToLower(URL.Raw)
			if !strings.HasPrefix(lowerURL, httpPrefix) &&
				!strings.HasPrefix(lowerURL, httpsPrefix) &&
				!strings.HasPrefix(lowerURL, ftpPrefix) &&
				!strings.Contains(lowerURL, "://") {
				URL.Raw = httpPrefix + URL.Raw
			}
			wuParse, err = wu.Parse(URL.Raw)
			if err != nil {
				return err
			}
		}
	} else {
		parsedURL, err := url.Parse(URL.Raw)
		if err != nil {
			return err
		}

		if parsedURL.IsAbs() {
			wuParse, err = wu.Parse(URL.Raw)
			if err != nil {
				return err
			}
		} else {
			baseURL := parentURL.GetParsed()
			if baseURL == nil {
				return fmt.Errorf("invalid baseURL in parentURL: %s", parentURL.Raw)
			}

			resolved := baseURL.ResolveReference(parsedURL)
			wuParse, err = wu.Parse(resolved.String())
			if err != nil {
				return err
			}
		}
	}

	wuParse.SetHash("")

	scheme := strings.ToLower(wuParse.Protocol())
	if scheme != "http:" && scheme != "https:" {
		return ErrUnsupportedScheme
	}

	// Check for localhost and 127.0.0.1
	host := wuParse.Hostname()
	if host == "localhost" || host == "127.0.0.1" {
		return ErrUnsupportedHost
	}

	// Check for TLD
	if !strings.Contains(host, ".") {
		return ErrUnsupportedHost
	}

	// Update the URL with the normalized version
	URL.Raw = wuParse.Href(false)

	return URL.Parse()
}
