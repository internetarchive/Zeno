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

func TransformURL(URL *url.URL) {
	// Add embed=1 to the URL, without changing the original URL
	if len(URL.RawQuery) > 0 {
		URL.RawQuery += "&embed=1&mode=tme"
	} else {
		URL.RawQuery = "embed=1&mode=tme"
	}
}
