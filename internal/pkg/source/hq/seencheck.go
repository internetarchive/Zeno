package hq

import (
	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gocrawlhq"
)

// SeencheckItem gets the MaxDepth children of the given item and sends a seencheck request to the crawl HQ for the URLs found.
// The items that were seen before will be marked as seen.
func SeencheckItem(item *models.Item) error {
	var URLsToSeencheck []gocrawlhq.URL

	items, err := item.GetNodesAtLevel(item.GetMaxDepth())
	if err != nil {
		panic(err)
	}

	// Never seencheck the seed
	if len(items) == 1 && items[0].IsSeed() {
		return nil
	}

	for i := range items {
		if items[i].IsSeed() {
			// Never seencheck the seed
			continue
		}

		if items[i].GetStatus() == models.ItemFresh {
			var source string
			if items[i].IsChild() {
				source = "asset"
			} else {
				source = "seed"
			}

			newURL := gocrawlhq.URL{
				Value: items[i].GetURL().Raw,
				Type:  source,
			}

			URLsToSeencheck = append(URLsToSeencheck, newURL)
		}
	}

	if len(URLsToSeencheck) == 0 {
		panic("no URLs to seencheck (can be caused if no fresh children were found)")
	}

	// Debug print the seencheck response
	for i := range URLsToSeencheck {
		logger.Debug("seencheck sent", "url", URLsToSeencheck[i].Value)
	}

	// Get seencheck URLs from CrawlHQ
	// If an URL is not returned it means that it was seen before
	outputURLs, err := globalHQ.client.Seencheck(URLsToSeencheck)
	if err != nil {
		return err
	}

	// Debug print the seencheck response
	if len(outputURLs) == 0 {
		logger.Debug("seencheck response is empty")
	}
	for i := range outputURLs {
		logger.Debug("seencheck response", "url", outputURLs[i].Value)
	}

	// For each child item, check if their URL was returned in the seencheck response. If not, mark them as seen.
	for i := range items {
		found := false
		for j := range outputURLs {
			if items[i].GetURL().String() == outputURLs[j].Value {
				found = true
				break
			}
		}

		if !found {
			items[i].SetStatus(models.ItemSeen)
		}
	}

	return nil
}
