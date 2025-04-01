package postprocessor

import (
	"slices"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/extractor"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/sitespecific/ina"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/sitespecific/truthsocial"
	"github.com/internetarchive/Zeno/pkg/models"
)

// extractAssets extracts assets from the item's body and returns them.
// It also potentially returns outlinks if the body contains URLs that are not assets.
func extractAssets(item *models.Item) (assets, outlinks []*models.URL, err error) {
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
			return assets, outlinks, err
		}

		HTMLAssets, err := extractor.HTMLAssets(item)
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return assets, outlinks, err
		}

		assets = append(INAAssets, HTMLAssets...)
	case truthsocial.NeedExtraction(item.GetURL()):
		assets, outlinks, err = truthsocial.ExtractAssets(item)
		if err != nil {
			logger.Error("unable to extract assets from TruthSocial", "err", err.Error(), "item", item.GetShortID())
			return assets, outlinks, err
		}
	case extractor.IsM3U8(item.GetURL()):
		assets, err = extractor.M3U8(item.GetURL())
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return assets, outlinks, err
		}
	case extractor.IsJSON(item.GetURL()):
		assets, outlinks, err = extractor.JSON(item.GetURL())
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return assets, outlinks, err
		}
	case extractor.IsXML(item.GetURL()):
		assets, outlinks, err = extractor.XML(item.GetURL())
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return assets, outlinks, err
		}
	case extractor.IsHTML(item.GetURL()):
		assets, err = extractor.HTMLAssets(item)
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return assets, outlinks, err
		}
	default:
		logger.Debug("no extractor used for page", "content-type", contentType, "item", item.GetShortID())
		return assets, outlinks, nil
	}

	for i := 0; i < len(assets); {
		asset := assets[i]

		// Case 1: asset is nil
		if asset == nil {
			logger.Debug("asset is nil, removing", "item", item.GetShortID())
			assets = slices.Delete(assets, i, i+1)
			continue // don't increment i, next item is now at same index
		}

		// Case 2: asset is a duplicate of the item's URL
		itemURL := item.GetURL()
		if itemURL != nil && asset.Raw == itemURL.String() {
			logger.Debug("removing asset that is a duplicate of the item URL",
				"item", item.GetShortID(), "asset", asset.Raw)
			assets = slices.Delete(assets, i, i+1)
			continue // same: skip increment to check the next item now at index i
		}

		// Nothing to delete → move to next item
		i++
	}

	// For assets, set the hops level to the item's level
	for _, asset := range assets {
		asset.SetHops(item.GetURL().GetHops())
	}

	// For outlinks, set the hops level to the item's level + 1
	for _, outlink := range outlinks {
		outlink.SetHops(item.GetURL().GetHops() + 1)
	}

	return assets, outlinks, nil
}

func shouldExtractAssets(item *models.Item) bool {
	return !config.Get().DisableAssetsCapture && item.GetURL().GetBody() != nil
}
