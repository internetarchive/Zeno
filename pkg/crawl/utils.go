package crawl

import (
	"github.com/CorentinB/Zeno/pkg/queue"
	"github.com/CorentinB/Zeno/pkg/utils"
	"mvdan.cc/xurls/v2"
	"net/http"
	"net/url"
	"strings"
)

func needBrowser(item *queue.Item) bool {
	res, err := http.Head(item.URL.String())
	if err != nil {
		return true
	}

	// If the Content-Type is text/html, we use a headless browser
	contentType := res.Header.Get("content-type")
	if strings.Contains(contentType, "text/html") {
		return true
	}

	return false
}

func extractOutlinks(source string) (outlinks []url.URL) {
	// Extract outlinks and dedupe them
	rxStrict := xurls.Strict()
	rawOutlinks := utils.DedupeStringSlice(rxStrict.FindAllString(source, -1))

	// Validate outlinks
	for _, outlink := range rawOutlinks {
		URL, err := url.Parse(outlink)
		if err != nil {
			continue
		}
		err = utils.ValidateURL(URL)
		if err != nil {
			continue
		}
		outlinks = append(outlinks, *URL)
	}

	return outlinks
}