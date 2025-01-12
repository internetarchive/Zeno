package postprocessor

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/extractor"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/sitespecific/ina"
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
	case ina.IsAPIURL(URL):
		assets, err := ina.ExtractMedias(URL)
		if err != nil {
			logger.Error("unable to extract medias from INA", "err", err.Error(), "item", item.GetShortID())
			return assets, err
		}
	case extractor.IsM3U8(URL):
		assets, err = extractor.M3U8(URL)
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return assets, err
		}
	case extractor.IsJSON(URL):
		assets, err = extractor.JSON(URL)
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return assets, err
		}
	case extractor.IsXML(URL):
		assets, err = extractor.XML(URL)
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return assets, err
		}
	case extractor.IsHTML(URL):
		assets, err = extractor.HTMLAssets(doc, URL, item)
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return assets, err
		}
	default:
		logger.Debug("no extractor used for page", "content-type", contentType, "item", item.GetShortID())
	}

	return
}
