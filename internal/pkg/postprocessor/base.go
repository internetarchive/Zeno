package postprocessor

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/pkg/models"
)

func scrapeBaseTag(item *models.Item) {
	item.GetURL().GetDocument().Find("base").Each(func(index int, base *goquery.Selection) {
		href, exists := base.Attr("href")
		if exists {
			item.SetBase(href)
		}

		return
	})
}
