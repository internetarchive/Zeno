package extractor

import (
	"bytes"
	"io"
	"net/http"
	// "strings"
	"testing"

	"github.com/internetarchive/Zeno/pkg/models"
	// "github.com/PuerkitoBio/goquery"
)

func TestHTMLOutlinks(t *testing.T) {
	body := `
	<html>
		<head></head>
		<body>
			<a href="http://example.com">ex</a>
			<a href="http://archive.org">ar</a>
			<p>test</p>
			<a href="https://web.archive.org">wa</a>
		</body>
	</html>
	`
	// alternative that also doesn't work
	// doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	// if err != nil {
	// 	t.Errorf("html doc loading failed %s", err)
	// }

	resp := &http.Response{
		Body: io.NopCloser(bytes.NewBufferString(body)),
  }
	newURL := &models.URL{Raw: "http://ex.com"}
  newURL.SetResponse(resp)
	//	newURL.SetDocument(doc)
	item := models.NewItem("test", newURL, "", false)
	outlinks, err := HTMLOutlinks(item)
	// also this doesn't work with a similar error.
	// outlinks, err := HTMLAssets(item)
	if err != nil {
		t.Errorf("Error extracting HTML outlinks %s", err)
	}
	if len(outlinks) != 3 {
		t.Errorf("We couldn't extract all HTML outlinks.")
	}
}
