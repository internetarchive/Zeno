package extractor

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/sitespecific/reddit"
	"github.com/internetarchive/Zeno/pkg/models"
)

func IsJSON(URL *models.URL) bool {
	return isContentType(URL.GetResponse().Header.Get("Content-Type"), "json")
}

func JSON(URL *models.URL) (assets []*models.URL, err error) {
	defer URL.RewindBody()

	bodyBytes := make([]byte, URL.GetBody().Len())
	_, err = URL.GetBody().Read(bodyBytes)
	if err != nil {
		return nil, err
	}

	rawAssets, err := GetURLsFromJSON(bodyBytes)
	if err != nil {
		return nil, err
	}

	for _, rawAsset := range rawAssets {
		if reddit.IsRedditURL(URL) {
			rawAsset, err = url.QueryUnescape(strings.ReplaceAll(rawAsset, "amp;", ""))
			if err != nil {
				return nil, err
			}
		}
		assets = append(assets, &models.URL{
			Raw:  rawAsset,
			Hops: URL.GetHops() + 1,
		})
	}

	return assets, err
}

func GetURLsFromJSON(body []byte) ([]string, error) {
	var data interface{}
	err := json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}

	links := make([]string, 0)
	findURLs(data, &links)

	return links, nil
}

func findURLs(data interface{}, links *[]string) {
	switch v := data.(type) {
	case string:
		if isValidURL(v) {
			*links = append(*links, v)
		}
	case []interface{}:
		for _, element := range v {
			findURLs(element, links)
		}
	case map[string]interface{}:
		for _, value := range v {
			findURLs(value, links)
		}
	}
}

func isValidURL(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}
