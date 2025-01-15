package postprocessor

import (
	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/pkg/models"
)

func postprocessItem(item *models.Item) []*models.Item {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.postprocess.postprocessItem",
	})

	outlinks := make([]*models.Item, 0)

	if item.GetStatus() != models.ItemArchived {
		logger.Debug("item not archived, skipping", "item_id", item.GetShortID())
		return outlinks
	}

	// Verify if there is any redirection
	// TODO: execute assets redirection
	if isStatusCodeRedirect(item.GetURL().GetResponse().StatusCode) {
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
	// - the item is a child and the URL has more than one hop
	// - assets capture is disabled and domains crawl is disabled
	// - the item is a seed and the URL has more hops than the max allowed
	if item.IsChild() && item.GetURL().GetHops() > 1 {
		logger.Debug("item is child and URL has more than one hop", "item_id", item.GetShortID())
		item.SetStatus(models.ItemCompleted)
		return outlinks
	} else if config.Get().DisableAssetsCapture && !config.Get().DomainsCrawl {
		logger.Debug("assets capture and domains crawl are disabled", "item_id", item.GetShortID())
		item.SetStatus(models.ItemCompleted)
		return outlinks
	}

	if item.GetURL().GetResponse() != nil && item.GetURL().GetResponse().StatusCode == 200 {
		// Extract assets from the page
		if !config.Get().DisableAssetsCapture && item.GetURL().GetBody() != nil {
			assets, err := extractAssets(item)
			if err != nil {
				logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			}

			for _, asset := range assets {
				if asset == nil {
					logger.Warn("nil asset", "item", item.GetShortID())
					continue
				}

				newChild := models.NewItem(uuid.New().String(), asset, "", false)
				err = item.AddChild(newChild, models.ItemGotChildren)
				if err != nil {
					panic(err)
				}
			}
		}

		// Extract outlinks from the page
		if (config.Get().DomainsCrawl || ((item.IsSeed() || item.IsRedirection()) && item.GetURL().GetHops() < config.Get().MaxHops)) && item.GetURL().GetBody() != nil {
			newOutlinks, err := extractOutlinks(item)
			if err != nil {
				logger.Error("unable to extract outlinks", "err", err.Error(), "item", item.GetShortID())
				return outlinks
			}

			for i := range newOutlinks {
				if newOutlinks[i] == nil {
					logger.Warn("nil link", "item", item.GetShortID())
					continue
				}

				newOutlinkItem := models.NewItem(uuid.New().String(), newOutlinks[i], item.GetURL().String(), true)
				outlinks = append(outlinks, newOutlinkItem)
			}

			logger.Debug("extracted outlinks", "item", item.GetShortID(), "count", len(newOutlinks))
		}
	}

	// Make sure the goquery document's memory can be freed
	item.GetURL().SetDocument(nil)

	if item.GetStatus() != models.ItemGotChildren && item.GetStatus() != models.ItemGotRedirected {
		item.SetStatus(models.ItemCompleted)
	}

	return outlinks
}
