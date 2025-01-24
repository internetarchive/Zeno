package truthsocial

import (
	"regexp"

	"github.com/internetarchive/Zeno/pkg/models"
)

var (
	postURLRegex = regexp.MustCompile(`https:\/\/truthsocial\.com\/@[A-Za-z0-9_]+\/posts\/`)
	postIDRegex  = regexp.MustCompile(`https:\/\/truthsocial\.com\/@[A-Za-z0-9_]+\/posts\/(\d+)`)
)

func IsPostURL(URL *models.URL) bool {
	return postURLRegex.MatchString(URL.String())
}

func GenerateAssetsURLs(URL *models.URL) (assets []*models.URL, err error) {
	// Get the ID from the URL
	postID := postIDRegex.FindStringSubmatch(URL.String())
	if len(postID) != 2 {
		return nil, nil
	}

	// Generate the assets URLs
	assets = append(assets, &models.URL{
		Raw: "https://truthsocial.com/api/v1/statuses/" + postID[1],
	})

	return assets, nil
}
