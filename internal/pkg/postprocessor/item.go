package postprocessor

import (
	"strings"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/domainscrawl"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/extractor"
	"github.com/internetarchive/Zeno/pkg/models"
)

func shouldExtractOutlinks(item *models.Item) bool {
	// Bypass the hop count if we are domain crawling to ensure we don't miss an outlink from a domain we are interested in
	if domainscrawl.Enabled() && item.GetURL().GetBody() != nil {
		return true
	}

	// Match pure hops count
	if item.GetURL().GetHops() < config.Get().MaxHops && item.GetURL().GetBody() != nil {
		return true
	}

	return false
}

func postprocessItem(item *models.Item) []*models.Item {
	defer item.Close()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.postprocess.postprocessItem",
		"item_id":   item.GetShortID(),
	})

	outlinks := make([]*models.Item, 0)

	if item.GetStatus() != models.ItemArchived {
		logger.Debug("item not archived, skipping")
		return outlinks
	}

	logger.Debug("postprocessing item")

	// Handle redirections
	if item.GetURL().GetResponse() != nil &&
		isStatusCodeRedirect(item.GetURL().GetResponse().StatusCode) {

		logger.Debug("item is a redirection")

		if item.GetURL().GetRedirects() >= config.Get().MaxRedirect {
			logger.Warn("max redirects reached")
			item.SetStatus(models.ItemCompleted)
			return outlinks
		}

		newURL := &models.URL{
			Raw:       item.GetURL().GetResponse().Header.Get("Location"),
			Redirects: item.GetURL().GetRedirects() + 1,
			Hops:      item.GetURL().GetHops(),
		}

		child := models.NewItem(newURL, "")
		_ = item.AddChild(child, models.ItemGotRedirected)

		return outlinks
	}

	// Depth and asset capture checks
	if !domainscrawl.Enabled() && item.GetDepthWithoutRedirections() > 2 &&
		!extractor.IsEmbeddedCSS(item) {

		logger.Debug("depth exceeded")
		item.SetStatus(models.ItemCompleted)
		return outlinks
	}

	if !domainscrawl.Enabled() &&
		item.GetDepthWithoutRedirections() == 1 &&
		strings.Contains(item.GetURL().GetMIMEType().String(), "html") {

		logger.Debug("HTML extracted as asset, skipping")
		item.SetStatus(models.ItemCompleted)
		return outlinks
	}

	if config.Get().DisableAssetsCapture && !domainscrawl.Enabled() {
		logger.Debug("assets disabled")
		item.SetStatus(models.ItemCompleted)
		return outlinks
	}

	// Process successful items
	if (item.GetURL().GetResponse() != nil &&
		item.GetURL().GetResponse().StatusCode == 200) ||
		(item.GetURL().GetResponse() == nil &&
			item.GetURL().GetBody() != nil) {

		logger.Debug("item is a success")

		doAssets := shouldExtractAssets(item)
		doOutlinks := shouldExtractOutlinks(item)

		var outlinksFromAssets []*models.URL

		// Extract assets
		if doAssets {
			var assets []*models.URL
			var err error

			assets, outlinksFromAssets, err = ExtractAssetsOutlinks(item)
			if err != nil {
				logger.Error("unable to extract assets", "err", err.Error())
			} else {
				for _, asset := range assets {
					if asset == nil {
						continue
					}
					child := models.NewItem(asset, "")
					_ = item.AddChild(child, models.ItemGotChildren)
				}
			}
		}

		// Extract outlinks
		if doOutlinks {
			newOutlinks, err := extractOutlinks(item)
			if err != nil {
				logger.Error("unable to extract outlinks", "err", err.Error())
			} else {
				newOutlinks = append(newOutlinks, outlinksFromAssets...)

				for _, link := range newOutlinks {
					if link == nil {
						continue
					}

					if domainscrawl.Enabled() &&
						domainscrawl.Match(link.Raw) {
						link.SetHops(0)
					} else if domainscrawl.Enabled() &&
						!domainscrawl.Match(link.Raw) &&
						item.GetURL().GetHops() >= config.Get().MaxHops {
						continue
					}

					outlinks = append(
						outlinks,
						models.NewItem(link, item.GetURL().String()),
					)
				}
			}
		}
	}

	item.GetURL().SetDocumentCache(nil)

	if !item.HasChildren() &&
		!item.HasRedirection() &&
		item.GetStatus() != models.ItemFailed {

		item.SetStatus(models.ItemCompleted)
	}

	return outlinks
}
