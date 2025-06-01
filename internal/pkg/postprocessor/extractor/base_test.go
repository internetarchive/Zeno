package extractor

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/pkg/models"
)

func newDocumentWithBaseTag(base string) *goquery.Document {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`<html><head><base href="PLACEHOLDER" target="_blank"></head><body></body></html>`))
	if err != nil {
		panic(err)
	}
	doc.Find("base").First().SetAttr("href", base)

	return doc
}

func TestExtractBaseTag(t *testing.T) {
	doc := newDocumentWithBaseTag("http://example.com/something/")

	item := models.NewItem(&models.URL{
		Raw: "https://example.com/something/page.html",
	}, "")

	extractBaseTag(item, doc)

	if item.GetBase().String() != "http://example.com/something/" {
		t.Errorf("Cannot find html doc base.href")
	}
}
