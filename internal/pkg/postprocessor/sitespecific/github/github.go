package github

import (
	"regexp"
	"strings"

	"github.com/internetarchive/Zeno/internal/pkg/log"
)

var logger = log.NewFieldedLogger(&log.Fields{
	"component": "postprocessor.sitespecific.github",
})

// User avatars: avatars.githubusercontent.com
// Temporary link to attachments: private-user-images.githubusercontent.com
// Github frontend .css .js resources: github.githubassets.com
var githubAssetsDomains = regexp.MustCompile(`(?i)^https://[a-z-]*\.?(?:githubusercontent|githubassets)\.com/`)

// Attachment links shown to the user in the editor
var githubComUserAttachments = regexp.MustCompile(`(?i)^https://github\.com/user-attachments/`)

// Permanent links to attachments
var githubComAssets = regexp.MustCompile(`(?i)https://github\.com/[^/]+/[^/]+/assets/`)

var matchers = []*regexp.Regexp{
	githubAssetsDomains,
	githubComUserAttachments,
	githubComAssets,
}

// Many GitHub asset urls do not have a file extension, so we need to consider
// some specific patterns to identify them as assets.
func ShouldConsiderAsAsset(u string) bool {
	if !strings.Contains(u, "github") {
		return false
	}

	for i := range matchers {
		if matchers[i].MatchString(u) {
			logger.Debug("matched GitHub asset pattern", "pattern", matchers[i].String(), "url", u)
			return true
		}
	}

	return false
}
