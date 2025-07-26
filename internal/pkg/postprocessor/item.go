package postprocessor

import (
	"net/url"
	"strings"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/domainscrawl"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/extractor"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/sitespecific/reddit"
	"github.com/internetarchive/Zeno/pkg/models"
)

func postprocessItem(item *models.Item) []*models.Item {
	defer item.Close()

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

		newChild := models.NewItem(newURL, "")
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
	// 			facebook.GenerateEmbedURL(item.GetURL()),
	// 			item.GetURL().String(),
	// 		), models.ItemGotChildren)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// }

	// Return if:
	// 1. the item [is not an embeded css item] and [is a child has a depth (without redirections) bigger than 2].
	//    -> we don't want to go too deep but still get the assets of assets (f.ex: m3u8)
	//    -> CSS @import chains can be very long, the depth control logic for embedded CSS item is in the AddAtImportLinksToItemChild() function separately.
	// 2. assets capture and domains crawl are disabled
	if !domainscrawl.Enabled() && item.GetDepthWithoutRedirections() > 2 && !extractor.IsEmbeddedCSS(item) {
		logger.Debug("item is a child and it's depth (without redirections) is more than 2", "item_id", item.GetShortID())
		item.SetStatus(models.ItemCompleted)
		return outlinks
	} else if !domainscrawl.Enabled() && (item.GetDepthWithoutRedirections() == 1 && strings.Contains(item.GetURL().GetMIMEType().String(), "html")) {
		logger.Debug("HTML got extracted as asset, skipping", "item_id", item.GetShortID())
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

			assets, outlinksFromAssets, err = ExtractAssetsOutlinks(item)
			if err != nil {
				logger.Error("unable to extract assets", "err", err.Error(), "item_id", item.GetShortID())
			} else {
				for i := range assets {
					if assets[i] == nil {
						logger.Warn("nil asset", "item", item.GetShortID())
						continue
					}

					// This is required to work around quirks in Reddit's URL encoding.
					if reddit.IsRedditURL(item.GetURL()) {
						unescaped, err := url.QueryUnescape(strings.ReplaceAll(assets[i].Raw, "amp;", ""))

						if err != nil {
							logger.Warn("reddit url unescapable", "item", item.GetShortID(), "asset", assets[i])
							continue
						}

						assets[i] = &models.URL{
							Raw:  unescaped,
							Hops: assets[i].Hops,
						}
					}

					newChild := models.NewItem(assets[i], "")
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

					newOutlinkItem := models.NewItem(newOutlinks[i], item.GetURL().String())
					outlinks = append(outlinks, newOutlinkItem)
				}

				logger.Debug("extracted outlinks", "item_id", item.GetShortID(), "count", len(newOutlinks))
			}
		}
	}

	// Make sure the goquery document's memory can be freed
	item.GetURL().SetDocumentCache(nil)

	if !item.HasChildren() && !item.HasRedirection() && item.GetStatus() != models.ItemFailed {
		logger.Debug("item has no children, setting as completed", "item_id", item.GetShortID())
		item.SetStatus(models.ItemCompleted)
	}

	return outlinks
}
