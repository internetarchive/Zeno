package extractor

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/clbanning/mxj/v2"
)

func XML(resp *http.Response) (URLs []*url.URL, err error) {
	xmlBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	mv, err := mxj.NewMapXml(xmlBody)
	if err != nil {
		return nil, err
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

	return URLs, nil
}
