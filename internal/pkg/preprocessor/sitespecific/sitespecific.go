package sitespecific

import (
	"net/http"

	"github.com/internetarchive/Zeno/internal/pkg/preprocessor/sitespecific/npr"
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor/sitespecific/reddit"
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor/sitespecific/tiktok"
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor/sitespecific/truthsocial"
	"github.com/internetarchive/Zeno/pkg/models"
)

type Preprocessor interface {
	Match(*models.URL) bool
	Apply(*http.Request)
}

var preprocessors = []Preprocessor{
	npr.NPRPreprocessor{},
	reddit.RedditPreprocessor{},
	tiktok.TikTokPreprocessor{},
	truthsocial.TruthsocialStatusPreprocessor{},
	truthsocial.TruthsocialAccountsPreprocessor{},
}

// Apply the first matching preprocessor.
func RunPreprocessors(URL *models.URL, req *http.Request) {
	for _, p := range preprocessors {
		if p.Match(URL) {
			p.Apply(req)
			break
		}
	}
}
