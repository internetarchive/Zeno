package hq

import (
	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gocrawlhq"
)

// SeencheckItem gets the MaxDepth children of the given item and sends a seencheck request to the crawl HQ for the URLs found.
// The items that were seen before will be marked as seen.
// A local otter cache is used to avoid sending redundant seencheck requests to HQ for URLs that have already been checked.
func (s *HQ) SeencheckItem(item *models.Item) error {
	var URLsToSeencheck []gocrawlhq.URL

	items, err := item.GetNodesAtLevel(item.GetMaxDepth())
	if err != nil {
		panic(err)
	}

	// Never seencheck the seed
	if len(items) == 1 && items[0].IsSeed() {
		return nil
	}

	hasCache := s.seencheckCache != nil

	for i := range items {
		if items[i].IsSeed() {
			// Never seencheck the seed
			continue
		}

		if items[i].GetStatus() == models.ItemFresh {
			urlStr := items[i].GetURL().Raw

			var source string
			if items[i].IsChild() {
				source = "asset"
			} else {
				source = "seed"
			}

			// If the URL is already in the cache as seen, mark it immediately and skip the HQ request.
			// However, if it was only checked as an asset before and is now a seed, re-check with HQ.
			if hasCache {
				if entry, ok := s.seencheckCache.Get(urlStr); ok && entry.seen {
					if source == "seed" && entry.source == "asset" {
						logger.Debug("seencheck cache bypass: was asset, now seed", "url", urlStr)
					} else {
						logger.Debug("seencheck cache hit (seen)", "url", urlStr, "source", source)
						items[i].SetStatus(models.ItemSeen)
						continue
					}
				}
			}

			newURL := gocrawlhq.URL{
				Value: urlStr,
				Type:  source,
			}

			URLsToSeencheck = append(URLsToSeencheck, newURL)
		}
	}

	// If all URLs were resolved from cache, nothing to send to HQ
	if len(URLsToSeencheck) == 0 {
		logger.Debug("all URLs resolved from seencheck cache (or otherwise), no HQ request needed")
		return nil
	}

	// Debug print the seencheck request
	for i := range URLsToSeencheck {
		logger.Debug("seencheck sent", "url", URLsToSeencheck[i].Value)
	}

	// Get seencheck URLs from CrawlHQ
	// If an URL is not returned it means that it was seen before
	outputURLs, err := s.client.Seencheck(s.ctx, URLsToSeencheck)
	if err != nil {
		return err
	}

	// Build a set of returned (not-seen) URLs for fast lookup
	notSeenSet := make(map[string]struct{}, len(outputURLs))
	for i := range outputURLs {
		logger.Debug("seencheck response", "url", outputURLs[i].Value)
		notSeenSet[outputURLs[i].Value] = struct{}{}
	}

	if len(outputURLs) == 0 {
		logger.Debug("seencheck response is empty")
	}

	// For each child item, check if their URL was returned in the seencheck response. If not, mark them as seen.
	for i := range items {
		if items[i].GetStatus() == models.ItemSeen {
			// Already marked (e.g. from cache), skip
			continue
		}

		urlStr := items[i].GetURL().String()
		_, notSeen := notSeenSet[urlStr]

		if !notSeen {
			items[i].SetStatus(models.ItemSeen)
		}

		// Update the cache with the result and the source type
		if hasCache {
			var src string
			if items[i].IsChild() {
				src = "asset"
			} else {
				src = "seed"
			}
			s.seencheckCache.Set(urlStr, seencheckCacheEntry{seen: !notSeen, source: src})
		}
	}

	return nil
}
