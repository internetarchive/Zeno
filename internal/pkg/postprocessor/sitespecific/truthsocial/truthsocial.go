package truthsocial

import (
	"regexp"

	"github.com/internetarchive/Zeno/pkg/models"
)

var (
	postURLRegex     = regexp.MustCompile(`^https?:\/\/truthsocial\.com\/@[A-Za-z0-9_]+\/posts\/`)
	postIDRegex      = regexp.MustCompile(`^https?:\/\/truthsocial\.com\/@[A-Za-z0-9_]+\/posts\/(\d+)`)
	usernameRegex    = regexp.MustCompile(`^https?:\/\/truthsocial\.com\/@([^/]+)`)
	statusesRegex    = regexp.MustCompile(`^https?:\/\/truthsocial\.com\/api\/v1\/statuses\/\d+$`)
	truthsocialRegex = regexp.MustCompile(`^https?:\/\/truthsocial\.com\/.*`)
)

func IsURL(URL *models.URL) bool {
	return truthsocialRegex.MatchString(URL.String())
}

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

func GenerateOutlinksURLs(URL *models.URL) (outlinks []*models.URL, err error) {
	// Get the username from the URL
	username := usernameRegex.FindStringSubmatch(URL.String())
	if len(username) != 2 {
		return nil, nil
	}

	// Generate the outlinks URLs
	outlinks = append(outlinks,
		&models.URL{
			Raw: "https://truthsocial.com/api/v1/accounts/lookup?acct=" + username[1],
		},
		&models.URL{
			Raw: "https://truthsocial.com/api/v1/accounts/" + username[1] + "/statuses?exclude_replies=true&only_replies=false&with_muted=true",
		},
		&models.URL{
			Raw: "https://truthsocial.com/api/v1/accounts/" + username[1] + "/statuses?pinned=true&only_replies=false&with_muted=true",
		},
		&models.URL{
			Raw: "https://truthsocial.com/api/v1/accounts/" + username[1] + "/statuses?with_muted=true&only_media=true",
		},
	)

	return outlinks, nil
}
