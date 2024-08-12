package youtube

import (
	"io"
	"net/url"

	"github.com/internetarchive/Zeno/internal/pkg/crawl/dependencies/ytdlp"
)

func Parse(body io.ReadCloser) (URLs []*url.URL, err error) {
	// Create a temporary server to serve the body and call ytdlp on it
	port, stopChan, err := ytdlp.ServeBody(body)
	if err != nil {
		return nil, err
	}
	defer close(stopChan)

	// Call ytdlp on the temporary server
	rawURLs, err := ytdlp.GetJSON(port)
	if err != nil {
		return nil, err
	}

	// Parse the URLs
	for _, urlString := range rawURLs {
		URL, err := url.Parse(urlString)
		if err != nil {
			return nil, err
		}

		URLs = append(URLs, URL)
	}

	return URLs, nil
}
