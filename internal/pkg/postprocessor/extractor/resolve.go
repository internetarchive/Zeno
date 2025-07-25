package extractor

import (
	"fmt"
	"net/url"

	"github.com/internetarchive/Zeno/pkg/models"
)

// resolveURL takes a URL string to resolve, a parent URL, and a base.
// If base is empty, it uses the parent URL as the base.
// It returns an absolute URL as a string.
func resolveURL(rawURL string, URL *models.URL) (absolute string, err error) {
	// Determine the base URL.
	var baseURL *url.URL
	if URL.GetBase() == nil { // If no base is provided, use the parent URL.
		baseURL = URL.GetParsed()
	} else {
		baseURL = URL.GetBase()
	}

	// Parse the URL to resolve.
	link, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}

	// If the link is already absolute, return it.
	if link.IsAbs() {
		return link.String(), nil
	}

	// Resolve the relative URL against the base.
	// The net/url.ResolveReference method follows RFC 3986, handling
	// relative paths (including those starting with "/" or "../").
	return baseURL.ResolveReference(link).String(), nil
}
