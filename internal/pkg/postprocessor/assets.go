package postprocessor

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/extractor"
	"github.com/internetarchive/Zeno/pkg/models"
)

func extractAssets(doc *goquery.Document, URL *models.URL, item *models.Item) (assets []*models.URL, err error) {
	var (
		contentType = URL.GetResponse().Header.Get("Content-Type")
		logger      = log.NewFieldedLogger(&log.Fields{
			"component": "postprocessor.extractAssets",
		})
	)

	// Extract assets from the body using the appropriate extractor
	switch {
	// Order is important, we want to check for more specific things first,
	// as they may trigger more general extractors (e.g. HTML)
	case extractor.IsM3U8(URL):
		assets, err = extractor.M3U8(URL)
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return assets, err
		}
	case extractor.IsHTML(URL):
		assets, err = extractor.HTML(doc, URL, item)
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return assets, err
		}
	default:
		logger.Debug("no extractor used for page", "content-type", contentType, "item", item.GetShortID())
	}

	// Extract URLs from the body using regex
	// var URLs []string
	// for _, regex := range []*regexp.Regexp{extractor.LinkRegexStrict, extractor.LinkRegex} {
	// 	// Reset the read position to the beginning
	// 	URL.RewindBody()

	// 	// Second read
	// 	buf := make([]byte, URL.GetBody().Len())
	// 	if _, err := URL.GetBody().Read(buf); err != nil && err != io.EOF {
	// 		return assets, err
	// 	}

	// 	URLs = append(URLs, regex.FindAllString(string(buf), -1)...)
	// }

	// // Reset the read position to the beginning
	// URL.RewindBody()

	// for _, URL := range utils.DedupeStrings(URLs) {
	// 	assets = append(assets, &models.URL{
	// 		Raw:  URL,
	// 		Hops: item.URL.GetHops(),
	// 	})
	// }

	return
}
