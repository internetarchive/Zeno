package extractor

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/clbanning/mxj/v2"
)

func XML(resp *http.Response) (URLs []*url.URL, sitemap bool, err error) {
	xmlBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, sitemap, err
	}

	mv, err := mxj.NewMapXml(xmlBody)
	if err != nil {
		return nil, sitemap, err
	}

	// Try to find if it's a sitemap
	for _, node := range mv.LeafNodes() {
		if strings.Contains(node.Path, "sitemap") {
			sitemap = true
			break
		}
	}

	for _, value := range mv.LeafValues() {
		if _, ok := value.(string); ok {
			if strings.HasPrefix(value.(string), "http") {
				URL, err := url.Parse(value.(string))
				if err == nil {
					URLs = append(URLs, URL)
				}
			}
		}
	}

	return URLs, sitemap, nil
}
