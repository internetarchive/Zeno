package telegram

import (
	"net/url"
	"strings"
)

func IsTelegramEmbedURL(url string) bool {
	return strings.Contains(url, "/t.me/") && strings.Contains(url, "?embed=1")
}

func IsTelegramURL(url string) bool {
	return strings.Contains(url, "/t.me/")
}

func CreateEmbedURL(URL *url.URL) *url.URL {
	// Add embed=1 to the URL, without changing the original URL
	embedURL := *URL

	if len(embedURL.RawQuery) > 0 {
		embedURL.RawQuery += "&embed=1&mode=tme"
	} else {
		embedURL.RawQuery = "embed=1&mode=tme"
	}

	return &embedURL
}
