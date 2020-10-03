package crawl

import (
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/sirupsen/logrus"
)

var regexOutlinks *regexp.Regexp

func (crawl *Crawl) setupCloseHandler() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		logrus.Warning("CTRL+C catched.. cleaning up and exiting.")
		crawl.Finish()
		os.Exit(0)
	}()
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

func extractOutlinksRegex(source string) (outlinks []url.URL) {
	// Extract outlinks and dedupe them
	rawOutlinks := utils.DedupeStringSlice(regexOutlinks.FindAllString(source, -1))

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
