package preprocessor

import (
	"net/url"
	"strings"

	"github.com/ada-url/goada"
	"github.com/internetarchive/Zeno/pkg/models"
	"golang.org/x/text/encoding"
)

// Normalize the URL by removing fragments, attempting to add URL scheme if missing,
// and converting relative URLs into absolute URLs. An error is returned if the URL
// cannot be normalized.
// If provide docEnc, it will be used to encode the URL query parameters.
func NormalizeURL(URL *models.URL, parentURL *models.URL, docEnc encoding.Encoding) (err error) {
	// Clean the URL by removing leading and trailing quotes
	URL.Raw = strings.Trim(URL.Raw, `"'`)

	var adaParse *goada.Url

	parsedURL, err := url.Parse(URL.Raw)
	if err != nil {
		return err
	}

	// Handle non-utf8 document encodings
	if docEnc != nil && docEnc != encoding.Nop {
		// According to the URL spec, we only need to encode the query part.
		// The path part should be left as utf8, we don't need to encode it.
		query := parsedURL.Query()
		newQuery := url.Values{}
		for key, values := range query {
			for _, value := range values {
				encodedKey, err := docEnc.NewEncoder().String(key)
				if err != nil {
					return err
				}
				encodedValue, err := docEnc.NewEncoder().String(value)
				if err != nil {
					return err
				}
				newQuery.Add(encodedKey, encodedValue)
			}
		}
		parsedURL.RawQuery = newQuery.Encode()
		URL.Raw = parsedURL.String()
	}

	if parentURL != nil && !parsedURL.IsAbs() {
		// Determine the base with the following logic:
		// - always with the <base> tag found in the HTML document, if it exists (TBI)
		// - if the URL starts with a slash, use the parent URL's scheme and host
		// - if the URL does not start with a slash, use the parent URL's scheme, host, and path
		baseURL := parentURL.GetParsed()
		if strings.HasPrefix(parsedURL.Path, "/") {
			adaParse, err = goada.NewWithBase(URL.Raw, baseURL.Scheme+"://"+baseURL.Host)
			if err != nil {
				return err
			}
		} else {
			adaParse, err = goada.NewWithBase(URL.Raw, baseURL.String())
			if err != nil {
				return err
			}
		}
	} else {
		if parsedURL.Scheme == "" {
			parsedURL.Scheme = "http"
		}

		adaParse, err = goada.New(models.URLToString(parsedURL))
		if err != nil {
			return err
		}
	}

	adaParse.SetHash("")
	if scheme := adaParse.Protocol(); scheme != "http:" && scheme != "https:" {
		return ErrUnsupportedScheme
	}

	// Check for localhost and 127.0.0.1
	host := adaParse.Hostname()
	if host == "localhost" || host == "127.0.0.1" {
		return ErrUnsupportedHost
	}

	// Check for TLD
	if !strings.Contains(host, ".") {
		return ErrUnsupportedHost
	}

	URL.Raw = adaParse.Href()
	adaParse.Free()

	return URL.Parse()
}
