package youtube

import (
	"io"
	"net/url"

	"github.com/internetarchive/Zeno/internal/pkg/crawl/dependencies/ytdlp"
)

func Parse(body io.ReadCloser) (URLs []*url.URL, rawJSON string, HTTPHeaders ytdlp.HTTPHeaders, err error) {
	// Create a temporary server to serve the body and call ytdlp on it
	port, stopChan, err := ytdlp.ServeBody(body)
	if err != nil {
		return nil, rawJSON, HTTPHeaders, err
	}
	defer close(stopChan)

	// Call ytdlp on the temporary server
	rawURLs, rawJSON, HTTPHeaders, err := ytdlp.GetJSON(port)
	if err != nil {
		return nil, rawJSON, HTTPHeaders, err
	}

	// Parse the URLs
	for _, urlString := range rawURLs {
		URL, err := url.Parse(urlString)
		if err != nil {
			return nil, rawJSON, HTTPHeaders, err
		}

		URLs = append(URLs, URL)
	}

	return URLs, rawJSON, HTTPHeaders, nil
}
