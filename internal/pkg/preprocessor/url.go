package preprocessor

import (
	"net/url"
	"strings"

	"github.com/ada-url/goada"
	"github.com/internetarchive/Zeno/pkg/models"
)

// Normalize the URL by removing fragments, attempting to add URL scheme if missing,
// and converting relative URLs into absolute URLs. An error is returned if the URL
// cannot be normalized.
func normalizeURL(URL *models.URL, parentURL *models.URL) (err error) {
	// Clean the URL by removing leading and trailing quotes
	URL.Raw = strings.Trim(URL.Raw, `"'`)

	var adaParse *goada.Url

	parsedURL, err := url.Parse(URL.Raw)
	if err != nil {
		return err
	}

	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "http"
	}

	if parentURL != nil && !parsedURL.IsAbs() {
		adaParse, err = goada.NewWithBase(URL.Raw, parentURL.String())
		if err != nil {
			return err
		}
	} else {
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

	return URL.Parse()
}
