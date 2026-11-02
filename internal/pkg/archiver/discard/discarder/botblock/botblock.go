// Package botblock detects responses produced by bot-blocking services
// (challenge, captcha and block pages).
//
// Detection rules are adapted from Heritrix's BotBlockDetector, but only use
// the status code and headers so that the body is never read:
// https://github.com/internetarchive/heritrix3/blob/838523d33ab180d837d8726b34b0f3142df0363f/modules/src/main/java/org/archive/modules/processor/BotBlockDetector.java
//
// Detected responses are not archived, so only signals that are specific to
// bot block pages are used, not heuristics. As a result, Incapsula and the
// Cloudflare and CloudFront block pages (as opposed to challenges) are not
// detected, as they have no such signal in their headers.
package botblock

import (
	"net/http"
	"strings"
)

const reasonPrefix = "Detected bot block page: "

type detector struct {
	service string
	detect  func(resp *http.Response) bool
}

// detectors are evaluated in order; the first match wins.
var detectors = []detector{
	{"akamai", detectAkamai},
	{"anubis", detectAnubis},
	{"aws-waf", detectAWSWAF},
	{"cloudflare", detectCloudflare},
	{"datadome", detectDataDome},
	{"kasada", detectKasada},
	{"lua-anti-ddos", detectLuaAntiDDoS},
	{"perimeterx", detectPerimeterX},
}

// Hook is a discard hook that discards responses from bot-blocking services.
func Hook(resp *http.Response) (bool, string) {
	if service := Detect(resp); service != "" {
		return true, reasonPrefix + service
	}
	return false, ""
}

// Detect returns the name of the bot-blocking service that produced resp, or "" if none.
func Detect(resp *http.Response) string {
	for _, d := range detectors {
		if d.detect(resp) {
			return d.service
		}
	}
	return ""
}

// Service returns the service name contained in a reason returned by Hook.
func Service(reason string) (string, bool) {
	return strings.CutPrefix(reason, reasonPrefix)
}

func detectAkamai(resp *http.Response) bool {
	return resp.StatusCode == 403 && resp.Header.Get("Server") == "AkamaiGHost"
}

func detectAnubis(resp *http.Response) bool {
	if resp.StatusCode == 307 && strings.Contains(resp.Header.Get("Location"), "/.within.website/?redir=") {
		return true
	}
	// Challenge pages set the verification cookie, whatever their (configurable) status code.
	// Other responses only clear these cookies or set the techaro.lol-anubis auth/referer
	// cookies, which lack the "-cookie-" part.
	// https://github.com/TecharoHQ/anubis/blob/f6595fb4acd13113eac0ade812df92eb41ed20af/lib/http.go#L335-L341
	for _, c := range resp.Cookies() {
		if strings.HasPrefix(c.Name, "techaro.lol-anubis-cookie-") && c.Value != "" && c.MaxAge >= 0 {
			return true
		}
	}
	return false
}

func detectAWSWAF(resp *http.Response) bool {
	// challenge or captcha page
	// https://docs.aws.amazon.com/waf/latest/developerguide/waf-captcha-and-challenge-actions.html
	return resp.Header.Get("X-Amzn-Waf-Action") != ""
}

func detectCloudflare(resp *http.Response) bool {
	// challenge page
	// https://developers.cloudflare.com/cloudflare-challenges/challenge-types/challenge-pages/detect-response/
	return strings.EqualFold(resp.Header.Get("Cf-Mitigated"), "challenge")
}

// X-Dd-B is set on block pages, e.g. https://github.com/pantherale0/pyfuelprices/issues/96
func detectDataDome(resp *http.Response) bool {
	return resp.Header.Get("X-Dd-B") != "" &&
		strings.EqualFold(resp.Header.Get("X-Datadome"), "protected")
}

func detectKasada(resp *http.Response) bool {
	return resp.StatusCode == 429 && resp.Header.Get("X-Kpsdk-Ct") != ""
}

// detectLuaAntiDDoS detects https://github.com/C0nw0nk/Nginx-Lua-Anti-DDoS
//
// The challenge page is the only response that sets X-Anti-DDoS (unless credits are disabled).
// Its status code isn't checked, as it has been 200 since 2025.
// https://github.com/C0nw0nk/Nginx-Lua-Anti-DDoS/blob/626a31710292988f43e22f92131d4ebb6f7658e6/lua/anti_ddos_challenge.lua#L8456-L8458
func detectLuaAntiDDoS(resp *http.Response) bool {
	return strings.HasPrefix(resp.Header.Get("X-Anti-DDoS"), "Conor McKnight")
}

// X-Px-Blocked is kept from Heritrix, though it doesn't appear in HUMAN's open-source enforcers.
// Their block pages have no other specific header: the _px cookies are also set on allowed pages.
func detectPerimeterX(resp *http.Response) bool {
	return (resp.StatusCode == 403 || resp.StatusCode == 429) &&
		resp.Header.Get("X-Px-Blocked") != ""
}
