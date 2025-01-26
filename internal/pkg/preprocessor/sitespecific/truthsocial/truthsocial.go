package truthsocial

import (
	"net/http"
	"regexp"

	"github.com/internetarchive/Zeno/pkg/models"
)

var (
	APIStatusRegex   = regexp.MustCompile(`^https?:\/\/truthsocial\.com\/api\/v1\/statuses\/(\d+)`)
	APIVideoRegex    = regexp.MustCompile(`^https?:\/\/truthsocial\.com\/api\/v1\/truth\/videos\/[a-zA-Z0-9]+$`)
	APIAccountsRegex = regexp.MustCompile(`^https?:\/\/truthsocial\.com\/api\/v1\/accounts\/([^/]+)`)
)

func IsVideoAPIURL(URL *models.URL) bool {
	return APIVideoRegex.MatchString(URL.String())
}

func IsStatusAPIURL(URL *models.URL) bool {
	return APIStatusRegex.MatchString(URL.String())
}

func IsAccountsAPIURL(URL *models.URL) bool {
	return APIAccountsRegex.MatchString(URL.String())
}

func AddAccountsAPIHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:134.0) Gecko/20100101 Firefox/134.0")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US;q=0.5,en;q=0.3")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("TE", "trailers")
}

func AddStatusAPIHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:134.0) Gecko/20100101 Firefox/134.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US;q=0.5,en;q=0.3")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Connection", "keep-alive")
}
