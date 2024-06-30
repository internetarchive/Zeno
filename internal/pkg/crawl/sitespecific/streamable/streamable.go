package streamable

import (
	"net/url"
	"strings"
)

func IsStreamableURL(URL string) bool {
	return strings.Contains(URL, "//streamable.com/")
}

func EmbedURLs() (URLs []*url.URL, err error) {
	URLsString := []string{
		"https://ui-statics-cf.streamable.com/player/_next/static/chunks/4684.ccb5ae368bff3fcc.js",
		"https://ui-statics-cf.streamable.com/player/_next/static/chunks/22e7f3a7.05a2bf84ec78694f.js",
	}

	for _, URL := range URLsString {
		apiURL, err := url.Parse(URL)
		if err != nil {
			return nil, err
		}

		URLs = append(URLs, apiURL)
	}

	return URLs, nil
}
