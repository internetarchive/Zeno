package truthsocial

import (
	"regexp"

	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/extractor"
	"github.com/internetarchive/Zeno/pkg/models"
)

var (
	postURLRegex       = regexp.MustCompile(`^https?:\/\/truthsocial\.com\/@[A-Za-z0-9_]+\/posts\/`)
	postIDRegex        = regexp.MustCompile(`^https?:\/\/truthsocial\.com\/@[A-Za-z0-9_]+\/posts\/(\d+)`)
	usernameRegex      = regexp.MustCompile(`^https?:\/\/truthsocial\.com\/@([^/]+)`)
	statusesRegex      = regexp.MustCompile(`^https?:\/\/truthsocial\.com\/api\/v1\/statuses\/\d+$`)
	accountLookupRegex = regexp.MustCompile(`^https?:\/\/truthsocial\.com\/api\/v1\/accounts\/lookup\?acct=[a-zA-Z0-9]+$`)
	truthsocialRegex   = regexp.MustCompile(`^https?:\/\/truthsocial\.com\/.*`)
)

func NeedExtraction(URL *models.URL) bool {
	return IsStatusesURL(URL) || IsPostURL(URL)
}

func ExtractAssets(item *models.Item) (assets []*models.URL, err error) {
	if IsStatusesURL(item.GetURL()) {
		truthsocialAssets, err := GenerateVideoURLsFromStatusesAPI(item.GetURL())
		if err != nil {
			return assets, err
		}

		JSONAssets, err := extractor.JSON(item.GetURL())
		if err != nil {
			return assets, err
		}

		assets = append(truthsocialAssets, JSONAssets...)
	} else if IsPostURL(item.GetURL()) {
		truthsocialAssets, err := GeneratePostAssetsURLs(item.GetURL())
		if err != nil {
			return assets, err
		}

		HTMLAssets, err := extractor.HTMLAssets(item)
		if err != nil {
			return assets, err
		}

		assets = append(truthsocialAssets, HTMLAssets...)
	}

	return assets, nil
}
