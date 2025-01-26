package truthsocial

import "github.com/internetarchive/Zeno/pkg/models"

func IsPostURL(URL *models.URL) bool {
	return postURLRegex.MatchString(URL.String())
}

func GeneratePostAssetsURLs(URL *models.URL) (assets []*models.URL, err error) {
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
