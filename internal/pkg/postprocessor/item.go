package postprocessor

import (
	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/domainscrawl"
	"github.com/internetarchive/Zeno/pkg/models"
)

func postprocessItem(item *models.Item) []*models.Item {
	defer closeBody(item)

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.postprocess.postprocessItem",
	})

	outlinks := make([]*models.Item, 0)

	if item.GetStatus() != models.ItemArchived {
		logger.Debug("item not archived, skipping", "item_id", item.GetShortID())
		return outlinks
	}

	logger.Debug("postprocessing item", "item_id", item.GetShortID())

	// Verify if there is any redirection
	if isStatusCodeRedirect(item.GetURL().GetResponse().StatusCode) {
		logger.Debug("item is a redirection", "item_id", item.GetShortID())

		// Check if the current redirections count doesn't exceed the max allowed
		if item.GetURL().GetRedirects() >= config.Get().MaxRedirect {
			logger.Warn("max redirects reached", "item_id", item.GetShortID())
			item.SetStatus(models.ItemCompleted)
			return outlinks
		}

		// Prepare the new item resulting from the redirection
		newURL := &models.URL{
			Raw:       item.GetURL().GetResponse().Header.Get("Location"),
			Redirects: item.GetURL().GetRedirects() + 1,
			Hops:      item.GetURL().GetHops(),
		}

		newChild := models.NewItem(uuid.New().String(), newURL, "", false)
		err := item.AddChild(newChild, models.ItemGotRedirected)
		if err != nil {
			panic(err)
		}

		return outlinks
	}

	// Execute site-specific post-processing
	// TODO: re-add, but it was causing:
	// panic: preprocessor received item with status 4
	// switch {
	// case facebook.IsFacebookPostURL(item.GetURL()):
	// 	err := item.AddChild(
	// 		models.NewItem(
	// 			uuid.New().String(),
	// 			facebook.GenerateEmbedURL(item.GetURL()),
	// 			item.GetURL().String(),
	// 			false,
	// 		), models.ItemGotChildren)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// }

	// Return if:
	// 1. the item is a child has a depth (without redirections) bigger than 2 -> we don't want to go too deep but still get the assets of assets (f.ex: m3u8)
	// 2. assets capture and domains crawl are disabled
	if !domainscrawl.Enabled() && item.GetDepthWithoutRedirections() > 2 {
		logger.Debug("item is a child and it's depth (without redirections) is more than 2", "item_id", item.GetShortID())
		item.SetStatus(models.ItemCompleted)
		return outlinks
	} else if config.Get().DisableAssetsCapture && !domainscrawl.Enabled() {
		logger.Debug("assets capture and domains crawl are disabled", "item_id", item.GetShortID())
		item.SetStatus(models.ItemCompleted)
		return outlinks
	}

	if item.GetURL().GetResponse() != nil && item.GetURL().GetResponse().StatusCode == 200 {
		logger.Debug("item is a success", "item_id", item.GetShortID())

		var outlinksFromAssets []*models.URL

		// Extract assets from the page
		if shouldExtractAssets(item) {
			var assets []*models.URL
			var err error

			assets, outlinksFromAssets, err = extractAssets(item)
			if err != nil {
				logger.Error("unable to extract assets", "err", err.Error(), "item_id", item.GetShortID())
			} else {
				for i := range assets {
					if assets[i] == nil {
						logger.Warn("nil asset", "item", item.GetShortID())
						continue
					}

					newChild := models.NewItem(uuid.New().String(), assets[i], "", false)
					err = item.AddChild(newChild, models.ItemGotChildren)
					if err != nil {
						panic(err)
					}
				}

				logger.Debug("extracted assets", "item_id", item.GetShortID(), "count", len(assets))
			}
		}

		// Extract outlinks from the page
		if shouldExtractOutlinks(item) {
			newOutlinks, err := extractOutlinks(item)
			if err != nil {
				logger.Error("unable to extract outlinks", "err", err.Error(), "item_id", item.GetShortID())
			} else {
				// Append the outlinks found from the assets
				newOutlinks = append(newOutlinks, outlinksFromAssets...)

				for i := range newOutlinks {
					if newOutlinks[i] == nil {
						logger.Warn("nil link", "item_id", item.GetShortID())
						continue
					}

					// If domains crawl, and if the host of the new outlinks match the host of its parent
					// and if its parent is at hop 0, then we need to set the hop count to 0.
					// TODO: maybe be more flexible than a strict match
					if domainscrawl.Enabled() && domainscrawl.Match(newOutlinks[i].Raw) {
						logger.Debug("setting hop count to 0 (domains crawl)", "item_id", item.GetShortID(), "url", newOutlinks[i].Raw)
						newOutlinks[i].SetHops(0)
					} else if domainscrawl.Enabled() && !domainscrawl.Match(newOutlinks[i].Raw) && item.GetURL().GetHops() >= config.Get().MaxHops {
						logger.Debug("skipping outlink due to hop count", "item_id", item.GetShortID(), "url", newOutlinks[i].Raw)
						continue
					}

					newOutlinkItem := models.NewItem(uuid.New().String(), newOutlinks[i], item.GetURL().String(), true)
					outlinks = append(outlinks, newOutlinkItem)
				}

				logger.Debug("extracted outlinks", "item_id", item.GetShortID(), "count", len(newOutlinks))
			}
		}
	}

	// Make sure the goquery document's memory can be freed
	item.GetURL().SetDocument(nil)

	if !item.HasChildren() && !item.HasRedirection() && item.GetStatus() != models.ItemFailed {
		logger.Debug("item has no children, setting as completed", "item_id", item.GetShortID())
		item.SetStatus(models.ItemCompleted)
	}

	return outlinks
}
