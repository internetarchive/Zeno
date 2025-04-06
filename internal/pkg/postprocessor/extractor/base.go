package extractor

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/pkg/models"
)

func extractBaseTag(item *models.Item, doc *goquery.Document) {
	base, exists := doc.Find("base").First().Attr("href")
	if exists {
		item.SetBase(base)
	}
}
