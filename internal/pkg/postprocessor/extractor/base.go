package extractor

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/pkg/models"
)

func extractBaseTag(item *models.Item, doc *goquery.Document) {
	doc.Find("base").Each(func(index int, base *goquery.Selection) {
		href, exists := base.Attr("href")
		if exists {
			item.SetBase(href)
		}

		return
	})
}
