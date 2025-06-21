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
var githubAvatar = "avatars.githubusercontent.com/u/"

// github frontend .css .js resources
var githubAssets = "github.githubassets.com/"

// Attachment links shown to the user in the editor
var githubUserAttachments = "github.com/user-attachments/"

// Permanent links to attachments
var regexGithubAsset = regexp.MustCompile(`(?i)https://github\.com/[^/]+/[^/]+/assets/`)

// Temporary links to attachments
var githubPrivateUserImages = "private-user-images.githubusercontent.com/"

var matchers []any = []any{
	githubAvatar,
	githubAssets,
	githubUserAttachments,
	regexGithubAsset,
	githubPrivateUserImages,
}

// Many GitHub asset urls do not have a file extension, so we need to consider
// some specific patterns to identify them as assets.
func ShouldConsiderAsAsset(u string) bool {
	if !strings.Contains(u, "github") {
		return false
	}

	for _, matcher := range matchers {
		switch m := matcher.(type) {
		case string:
			if strings.Contains(u, m) {
				logger.Debug("matched GitHub asset pattern", "pattern", m, "url", u)
				return true
			}
		case *regexp.Regexp:
			if m.MatchString(u) {
				logger.Debug("matched GitHub asset pattern", "pattern", m.String(), "url", u)
				return true
			}
		}
	}

	return false
}
