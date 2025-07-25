package npr

import (
	"net/http"
	"strings"

	"github.com/internetarchive/Zeno/pkg/models"
)

type NPRPreprocessor struct{}

func (NPRPreprocessor) Match(URL *models.URL) bool {
	return strings.Contains(URL.String(), "npr.org/")
}

func (NPRPreprocessor) Apply(req *http.Request) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "fr,fr-FR;q=0.8,en-US;q=0.5,en;q=0.3")
	req.Header.Set("Referer", "https://www.npr.org/")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Priority", "u=0, i")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("TE", "trailers")
}
