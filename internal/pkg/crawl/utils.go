package crawl

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
)

var regexOutlinks *regexp.Regexp

func (crawl *Crawl) writeFrontierToDisk() {
	for !crawl.Finished.Get() {
		crawl.Frontier.Save()
		time.Sleep(time.Minute * 1)
	}
}

func needBrowser(item *frontier.Item) bool {
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
