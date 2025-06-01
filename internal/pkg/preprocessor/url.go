package preprocessor

import (
	"net/url"
	"strings"

	"github.com/internetarchive/Zeno/pkg/models"
)

// Normalize the URL by removing fragments, attempting to add URL scheme if missing,
// and converting relative URLs into absolute URLs. An error is returned if the URL
// cannot be normalized.
func NormalizeURL(URL *models.URL, parentURL *models.URL) (err error) {
	// Clean the URL by removing leading and trailing quotes
	URL.Raw = strings.Trim(URL.Raw, `"'`)

	parsedURL, err := url.Parse(URL.Raw)
	if err != nil {
		return err
	}

	// If parentURL is provided and parsedURL is relative, resolve against parent
	if parentURL != nil && !parsedURL.IsAbs() {
		baseURL := parentURL.GetParsed()
		baseParsed := &url.URL{
			Scheme: baseURL.Scheme,
			Host:   baseURL.Host,
			Path:   baseURL.Path,
		}
		parsedURL = baseParsed.ResolveReference(parsedURL)
	}

	// If scheme is missing, default to http and reparse
	if parsedURL.Scheme == "" {
		parsedURL, err = url.Parse("http://" + URL.Raw)
		if err != nil {
			return err
		}
	}

	parsedURL.Fragment = ""

	// Ensure path is "/" if empty
	if parsedURL.Path == "" {
		parsedURL.Path = "/"
	}

	// Only allow http and https
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return ErrUnsupportedScheme
	}

	// Check for localhost and 127.0.0.1
	host := parsedURL.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "" {
		return ErrUnsupportedHost
	}

	// Check for TLD in host
	if !strings.Contains(host, ".") {
		return ErrUnsupportedHost
	}

	// Assign normalized URL back
	URL.Raw = parsedURL.String()
	return URL.Parse()
}
