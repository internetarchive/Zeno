package extractor

import (
	"encoding/json"
	"strings"

	"github.com/ImVexed/fasturl"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/sitespecific/github"
	"github.com/internetarchive/Zeno/pkg/models"
)

func IsJSON(URL *models.URL) bool {
	return URL.GetMIMEType() != nil && strings.Contains(URL.GetMIMEType().String(), "json")
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
	var data any

	err = decoder.Decode(&data)
	if err != nil {
		return nil, nil, err
	}

	links := make([]string, 0)
	findURLs(data, &links)

	// We only consider as assets the URLs in which we can find a file extension
	for _, link := range links {
		if hasFileExtension(link) || github.ShouldConsiderAsAsset(link) {
			assets = append(assets, link)
		} else {
			outlinks = append(outlinks, link)
		}
	}

	return assets, outlinks, nil
}

func isLikelyJSON(str string) bool {
	// minimal json with a non-empty string
	// -> len(`["a"]`)
	if len(str) < 5 {
		return false
	}

	return ((str[0] == '{' && str[len(str)-1] == '}') || (str[0] == '[' && str[len(str)-1] == ']')) && strings.Contains(str, `"`)
}

func findURLs(data any, links *[]string) {
	switch v := data.(type) {
	case string:
		if isValidURL(v) {
			*links = append(*links, v)
			return
		} else if isLikelyJSON(v) {
			// handle JSON in JSON
			var jsonstringdata any
			err := json.Unmarshal([]byte(v), &jsonstringdata)
			if err == nil {
				findURLs(jsonstringdata, links)
				return
			}
		}

		// find links in text
		var linksFromText []string
		if !config.Get().StrictRegex {
			linksFromText = LinkRegex.FindAllString(v, -1)
		} else {
			linksFromText = LinkRegexStrict.FindAllString(v, -1)
		}
		for _, link := range linksFromText {
			if isValidURL(link) {
				*links = append(*links, link)
			}
		}
	case []any:
		for _, element := range v {
			findURLs(element, links)
		}
	case map[string]any:
		for _, value := range v {
			findURLs(value, links)
		}
	}
}

// This is a simplified version of the URL validation for quick checks.
func isValidURL(str string) bool {
	u, err := fasturl.ParseURL(str)
	if err != nil {
		return false
	}

	// A URL is considered valid if it has a scheme and a host, or if it has a host and a path/query.
	return (u.Protocol != "" && u.Host != "") || (u.Host != "" && (u.Path != "" || u.Query != ""))
}
