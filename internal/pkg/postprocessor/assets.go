package postprocessor

import (
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/extractor"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/sitespecific/ina"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/sitespecific/truthsocial"
	"github.com/internetarchive/Zeno/pkg/models"
)

func extractAssets(item *models.Item) (assets []*models.URL, err error) {
	var (
		contentType = item.GetURL().GetResponse().Header.Get("Content-Type")
		logger      = log.NewFieldedLogger(&log.Fields{
			"component": "postprocessor.extractAssets",
		})
	)

	// Extract assets from the body using the appropriate extractor
	switch {
	// Order is important, we want to check for more specific things first,
	// as they may trigger more general extractors (e.g. HTML)
	case ina.IsAPIURL(item.GetURL()):
		INAAssets, err := ina.ExtractMedias(item.GetURL())
		if err != nil {
			logger.Error("unable to extract medias from INA", "err", err.Error(), "item", item.GetShortID())
			return assets, err
		}

		HTMLAssets, err := extractor.HTMLAssets(item)
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return assets, err
		}

		assets = append(INAAssets, HTMLAssets...)
	case truthsocial.NeedExtraction(item.GetURL()):
		assets, err = truthsocial.ExtractAssets(item)
		if err != nil {
			logger.Error("unable to extract assets from TruthSocial", "err", err.Error(), "item", item.GetShortID())
			return assets, err
		}
	case extractor.IsM3U8(item.GetURL()):
		assets, err = extractor.M3U8(item.GetURL())
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return assets, err
		}
	case extractor.IsJSON(item.GetURL()):
		assets, err = extractor.JSON(item.GetURL())
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return assets, err
		}
	case extractor.IsXML(item.GetURL()):
		assets, err = extractor.XML(item.GetURL())
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return assets, err
		}
	case extractor.IsHTML(item.GetURL()):
		assets, err = extractor.HTMLAssets(item)
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return assets, err
		}
	default:
		logger.Debug("no extractor used for page", "content-type", contentType, "item", item.GetShortID())
		return assets, nil
	}

	// Set the hops level to the item's level
	for _, asset := range assets {
		asset.SetHops(item.GetURL().GetHops())
	}

	return assets, nil
}

func shouldExtractAssets(item *models.Item) bool {
	return !config.Get().DisableAssetsCapture && item.GetURL().GetBody() != nil
}
