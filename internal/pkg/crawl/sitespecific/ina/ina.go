package ina

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
)

type APIResponse struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	DateOfBroadcast time.Time `json:"dateOfBroadcast"`
	Type            string    `json:"type"`
	Duration        int       `json:"duration"`
	Categories      []any     `json:"categories"`
	Credits         []struct {
		Context struct {
			Vocab      string `json:"@vocab"`
			Hydra      string `json:"hydra"`
			Name       string `json:"name"`
			Value      string `json:"value"`
			Attributes string `json:"attributes"`
		} `json:"@context"`
		Type       string `json:"@type"`
		ID         string `json:"@id"`
		Name       string `json:"name"`
		Value      string `json:"value"`
		Attributes []struct {
			Context struct {
				Vocab string `json:"@vocab"`
				Hydra string `json:"hydra"`
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"@context"`
			Type  string `json:"@type"`
			ID    string `json:"@id"`
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"attributes"`
	} `json:"credits"`
	Restrictions                 []any  `json:"restrictions"`
	ResourceURL                  string `json:"resourceUrl"`
	ResourceThumbnail            string `json:"resourceThumbnail"`
	RestrictedBroadcastCountries []any  `json:"restrictedBroadcastCountries"`
	EmbedURL                     string `json:"embedUrl"`
	AllowEmbed                   bool   `json:"allowEmbed"`
	Ratio                        string `json:"ratio"`
	CollectionTitle              string `json:"collectionTitle"`
	IsOnline                     bool   `json:"isOnline"`
	AllowAds                     bool   `json:"allowAds"`
	TypeMedia                    string `json:"typeMedia"`
	HideLogo                     bool   `json:"hideLogo"`
	URI                          string `json:"uri"`
	AdvertisingAsset             bool   `json:"advertisingAsset"`
}

func IsURL(req *http.Request) bool {
	return strings.Contains(utils.URLToString(req.URL), "ina.fr")
}

func IsAPIURL(req *http.Request) bool {
	return strings.Contains(utils.URLToString(req.URL), "apipartner.ina.fr") && !strings.Contains(utils.URLToString(req.URL), "playerConfigurations.json")
}

func ExtractPlayerURLs(doc *goquery.Document) []*url.URL {
	var assets []string

	doc.Find("div[data-type=player]").Each(func(i int, s *goquery.Selection) {
		if playerConfigURL, exists := s.Attr("config-url"); exists {
			assets = append(assets, playerConfigURL)
		}

		if assetDetailsURL, exists := s.Attr("asset-details-url"); exists {
			assets = append(assets, assetDetailsURL)
		}

		if posterURL, exists := s.Attr("poster"); exists {
			assets = append(assets, posterURL)
		}
	})

	return utils.StringSliceToURLSlice(assets)
}

func ExtractMedias(resp *http.Response) ([]*url.URL, error) {
	var assets []string

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data APIResponse
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}

	assets = append(assets, data.ResourceURL, data.ResourceThumbnail, "https://player.ina.fr"+data.EmbedURL, data.URI)

	return utils.StringSliceToURLSlice(assets), nil
}
