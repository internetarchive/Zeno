package extractor

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/pkg/models"
)

func extractBaseTag(item *models.Item, doc *goquery.Document) {
	doc.Find("base").Each(func(index int, i *goquery.Selection) {
		base, exists := i.Attr("href")
		if exists {
			item.SetBase(base)
		}
	})
}
