package preprocessor

import (
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/pkg/models"
)

func matchRegexExclusion(item *models.Item) bool {
	for _, exclusion := range config.Get().ExclusionRegexes {
		if exclusion.MatchString(item.GetURL().String()) {
			return true
		}
	}

	return false
}
