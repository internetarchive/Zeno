package extractor

import (
	"encoding/json"
	"strings"

	"github.com/ImVexed/fasturl"
	"github.com/internetarchive/Zeno/pkg/models"
)

func IsJSON(URL *models.URL) bool {
	return isContentType(URL.GetResponse().Header.Get("Content-Type"), "json") || strings.Contains(URL.GetMIMEType().String(), "json")
}

func JSON(URL *models.URL) (assets, outlinks []*models.URL, err error) {
	defer URL.RewindBody()

	rawURLs, err := GetURLsFromJSON(json.NewDecoder(URL.GetBody()))
	if err != nil {
		return nil, nil, err
	}

	// We only consider as assets the URLs in which we can find a file extension
	for _, rawURL := range rawURLs {
		if hasFileExtension(rawURL) {
			assets = append(assets, &models.URL{
				Raw: rawURL,
			})
		} else {
			outlinks = append(outlinks, &models.URL{
				Raw: rawURL,
			})
		}
	}

	return assets, outlinks, nil
}

func GetURLsFromJSON(decoder *json.Decoder) ([]string, error) {
	var data interface{}
	err := decoder.Decode(&data)
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
	u, err := fasturl.ParseURL(str)
	return err == nil && u.Host != ""
}
