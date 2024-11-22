package postprocessor

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/pkg/models"
)

func scrapeBaseTag(doc *goquery.Document, item *models.Item) {
	doc.Find("base").Each(func(index int, base *goquery.Selection) {
		href, exists := base.Attr("href")
		if exists {
			item.SetChildsBase(href)
		}

		return
	})
}
