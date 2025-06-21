package github

import (
	"regexp"
	"strings"

	"github.com/internetarchive/Zeno/internal/pkg/log"
)

var logger = log.NewFieldedLogger(&log.Fields{
	"component": "postprocessor.sitespecific.github",
})

// user avatars
var githubAvatar = regexp.MustCompile(`(?i)^https://avatars\.githubusercontent\.com/u/`)

// github frontend .css .js resources
var githubFrontendAssets = regexp.MustCompile(`(?i)^https://github\.githubassets\.com/`)

// Attachment links shown to the user in the editor
var githubComUserAttachments = regexp.MustCompile(`(?i)^https://github\.com/user-attachments/`)

// Permanent links to attachments
var githubComAssets = regexp.MustCompile(`(?i)https://github\.com/[^/]+/[^/]+/assets/`)

// Temporary links to attachments
var githubPrivateUserImages = regexp.MustCompile(`(?i)^https://private-user-images\.githubusercontent\.com/`)

var matchers = []*regexp.Regexp{
	githubAvatar,
	githubFrontendAssets,
	githubComUserAttachments,
	githubComAssets,
	githubPrivateUserImages,
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
