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

	rawAssets, rawOutlinks, err := GetURLsFromJSON(json.NewDecoder(URL.GetBody()))
	if err != nil {
		return nil, nil, err
	}

	for _, rawAsset := range rawAssets {
		assets = append(assets, &models.URL{Raw: rawAsset})
	}

	for _, rawOutlink := range rawOutlinks {
		outlinks = append(outlinks, &models.URL{Raw: rawOutlink})
	}

	return assets, outlinks, nil
}

func GetURLsFromJSON(decoder *json.Decoder) (assets, outlinks []string, err error) {
	var data interface{}

	err = decoder.Decode(&data)
	if err != nil {
		return nil, nil, err
	}

	links := make([]string, 0)
	findURLs(data, &links)

	// We only consider as assets the URLs in which we can find a file extension
	for _, link := range links {
		if hasFileExtension(link) {
			assets = append(assets, link)
		} else {
			outlinks = append(outlinks, link)
		}
	}

	return assets, outlinks, nil
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
