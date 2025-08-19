package cloudflare

import (
	"net/http"

	"github.com/internetarchive/Zeno/internal/pkg/stats"
)

var ChallengeDetected = "Detected Cloudflare challenge page"

// ChallengePageHook detects Cloudflare challenge pages.
func ChallengePageHook(resp *http.Response) (bool, string) {
	if resp.StatusCode == 403 && resp.Header.Get("cf-mitigated") == "challenge" {
		stats.CFMitigatedIncr()
		return true, ChallengeDetected
	}
	return false, ""
}
