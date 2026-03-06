package akamai

import (
	"net/http"

	"github.com/internetarchive/Zeno/internal/pkg/stats"
)

var ChallengeDetected = "Detected Akamai challenge page"

// ChallengePageHook detects Akamai challenge pages.
func ChallengePageHook(resp *http.Response) (bool, string) {
	if resp.StatusCode == 403 && resp.Header.Get("Server") == "AkamaiGHost" {
		stats.AkamaiMitigatedIncr()
		return true, ChallengeDetected
	}
	return false, ""
}
