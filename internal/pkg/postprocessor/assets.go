package postprocessor

import (
	"net/url"
	"slices"
	"strings"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/extractor"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/sitespecific/ina"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/sitespecific/reddit"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/sitespecific/truthsocial"
	"github.com/internetarchive/Zeno/pkg/models"
)

// ExtractAssetsOutlinks extracts assets from the item's body and returns them.
// It also potentially returns outlinks if the body contains URLs that are not assets.
func ExtractAssetsOutlinks(item *models.Item) (assets, outlinks []*models.URL, err error) {
	assets, outlinks, err = Extractors(item)
	return SanitizeAssetsOutlinks(item, assets, outlinks, err)
}

// Extract assets and outlinks from the body using the appropriate extractor
// Order is important, we want to check for more specific things first,
// as they may trigger more general extractors (e.g. HTML)
// TODO this should be refactored using interfaces
func Extractors(item *models.Item) (assets, outlinks []*models.URL, err error) {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.Extractors",
		"item":      item.GetShortID(),
	})

	switch {
	case ina.IsAPIURL(item.GetURL()):
		INAAssets, err := ina.ExtractMedias(item.GetURL())
		if err != nil {
			logger.Error("unable to extract medias from INA", "err", err.Error())
			return assets, outlinks, err
		}

		HTMLAssets, err := extractor.HTMLAssets(item)
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error())
			return assets, outlinks, err
		}

		assets = append(INAAssets, HTMLAssets...)
	case truthsocial.NeedExtraction(item.GetURL()):
		assets, outlinks, err = truthsocial.ExtractAssets(item)
		if err != nil {
			logger.Error("unable to extract assets from TruthSocial", "err", err.Error())
			return assets, outlinks, err
		}
	case extractor.IsM3U8(item.GetURL()):
		assets, err = extractor.M3U8(item.GetURL())
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error())
			return assets, outlinks, err
		}
	case extractor.IsJSON(item.GetURL()):
		assets, outlinks, err = extractor.JSON(item.GetURL())
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error())
			return assets, outlinks, err
		}
	case extractor.IsXML(item.GetURL()):
		assets, outlinks, err = extractor.XML(item.GetURL())
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error())
			return assets, outlinks, err
		}
	case extractor.IsHTML(item.GetURL()):
		assets, err = extractor.HTMLAssets(item)
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error())
			return assets, outlinks, err
		}
	case extractor.IsEmbeddedCSS(item):
		var atImportLinks []*models.URL
		assets, atImportLinks, err = extractor.ExtractFromURLCSS(item.GetURL())

		logArgs := []any{"links", len(assets), "at_import_links", len(atImportLinks)}
		if err != nil {
			logArgs = append(logArgs, "err", err)
			logger.Error("error extracting assets from CSS", logArgs...)
		} else {
			logger.Debug("extracted assets from CSS", logArgs...)
		}
		extractor.AddAtImportLinksToItemChild(item, atImportLinks)
	default:
		contentType := item.GetURL().GetResponse().Header.Get("Content-Type")
		logger.Debug("no extractor used for page", "content-type", contentType, "mime", item.GetURL().GetMIMEType().String())
		return assets, outlinks, nil
	}

	return assets, outlinks, err
}

func SanitizeAssetsOutlinks(item *models.Item, assets []*models.URL, outlinks []*models.URL, err error) ([]*models.URL, []*models.URL, error) {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.SanitizeAssetsOutlinks",
		"item":      item.GetShortID(),
	})
	for i := 0; i < len(assets); {
		asset := assets[i]

		// Case 1: asset is nil
		if asset == nil {
			logger.Debug("asset is nil, removing")
			assets = slices.Delete(assets, i, i+1)
			continue // don't increment i, next item is now at same index
		}

		// Case 2: asset is a duplicate of the item's URL
		itemURL := item.GetURL()
		if itemURL != nil && asset.Raw == itemURL.String() {
			logger.Debug("removing asset that is a duplicate of the item URL", "asset", asset.Raw)
			assets = slices.Delete(assets, i, i+1)
			continue // same: skip increment to check the next item now at index i
		}

		// This is required to work around quirks in Reddit's URL encoding.
		if reddit.IsRedditURL(item.GetURL()) {
			unescaped, err := url.QueryUnescape(strings.ReplaceAll(asset.Raw, "amp;", ""))
			if err != nil {
				logger.Warn("reddit url unescapable", "item", item.GetShortID(), "asset", asset.Raw)
				continue
			}
			assets[i] = &models.URL{
				Raw:  unescaped,
				Hops: asset.Hops,
			}
		}

		// Nothing to delete → move to next item
		i++
	}

	assets, outlinks = filterURLsByProtocol(assets), filterURLsByProtocol(outlinks)

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

// 1. If Zeno is running in headless mode, we don't extract assets
// 2. If --disable-assets-capture is set, we don't extract assets
// 3. If the item.body is nil, we don't extract assets
func shouldExtractAssets(item *models.Item) bool {
	return !config.Get().Headless && !config.Get().DisableAssetsCapture && item.GetURL().GetBody() != nil
}
