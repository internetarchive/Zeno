package postprocessor

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/extractor"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/Zeno/pkg/models"
)

func extractAssets(doc *goquery.Document, URL *models.URL, item *models.Item) (err error) {
	var (
		assets      []*models.URL
		contentType = URL.GetResponse().Header.Get("Content-Type")
		logger      = log.NewFieldedLogger(&log.Fields{
			"component": "postprocessor.extractAssets",
		})
	)
	// Extract assets from the body using the appropriate extractor
	switch {
	case strings.Contains(contentType, "html"):
		assets, err = extractor.HTML(doc, URL, item)
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return err
		}
	default:
		logger.Debug("no extractor found for content type", "content-type", contentType, "item", item.GetShortID())
	}

	// Extract URLs from the body using regex
	var URLs []string
	for _, regex := range []*regexp.Regexp{extractor.LinkRegexStrict, extractor.LinkRegex} {
		URLs = append(URLs, regex.FindAllString(item.GetBody().String(), -1)...)
	}

	for _, URL := range utils.DedupeStrings(URLs) {
		assets = append(assets, &models.URL{
			Raw:  URL,
			Hops: item.URL.GetHops(),
		})
	}

	for _, asset := range assets {
		// If the item has a value of 0 for ChildsCaptured, it means that we are  on the first iteration
		// of the postprocessor and we allow another iteration to capture the assets of assets
		if item.GetChildsCaptured() == 0 {
			item.AddChild(asset)
		}
	}

	return
}
