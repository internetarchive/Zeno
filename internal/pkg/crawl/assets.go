package crawl

import (
	"net/url"

	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/PuerkitoBio/goquery"
)

func extractAssets(base *url.URL, doc *goquery.Document) (assets []url.URL, err error) {
	var rawAssets []string

	// Extract assets on the page (images, scripts, videos..)
	doc.Find("img").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("src")
		if exists {
			rawAssets = append(rawAssets, link)
		}
	})
	doc.Find("video").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("src")
		if exists {
			rawAssets = append(rawAssets, link)
		}
	})
	doc.Find("script").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("src")
		if exists {
			rawAssets = append(rawAssets, link)
		}
	})
	doc.Find("link").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("href")
		if exists {
			rawAssets = append(rawAssets, link)
		}
	})
	doc.Find("audio").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("src")
		if exists {
			rawAssets = append(rawAssets, link)
		}
	})
	doc.Find("iframe").Each(func(index int, item *goquery.Selection) {
		link, exists := item.Attr("src")
		if exists {
			rawAssets = append(rawAssets, link)
		}
	})

	// Turn strings into url.URL
	assets = utils.StringSliceToURLSlice(rawAssets)

	// Go over all assets and outlinks and make sure they are absolute links
	assets = utils.MakeAbsolute(base, assets)

	return utils.DedupeURLs(assets), nil
}
