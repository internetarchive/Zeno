package extractor

import (
	"strings"
	"testing"

	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/PuerkitoBio/goquery"
)

func TestExtractBaseTag(t *testing.T) {
	htmlString := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Test Page</title>
		<base href="http://example.com/something/"
	</head>
	<body>
		<p>First paragraph</p>
	</body>
	</html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlString))
	if err != nil {
		t.Errorf("html doc loading failed %s", err)
	}

	item := models.NewItem("test", &models.URL{
    Raw: "https://example.com/something/page.html",
  }, "")

	extractBaseTag(item, doc)

	if item.GetBase() != "http://example.com/something/" {
		t.Errorf("Cannot find html doc base.href")
	}
}
