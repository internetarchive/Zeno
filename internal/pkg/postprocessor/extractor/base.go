package extractor

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/pkg/models"
)

var basetagLogger = log.NewFieldedLogger(&log.Fields{
	"component": "postprocessor.extractor.base",
})

// extract document <base> tag and set the base URL for item if it's valid
func extractBaseTag(item *models.Item, doc *goquery.Document) {
	// spec ref: https://html.spec.whatwg.org/multipage/semantics.html#the-base-element
	base, exists := doc.Find("base").First().Attr("href")
	if !exists {
		return
	}

	// https://html.spec.whatwg.org/multipage/urls-and-fetching.html#valid-url-potentially-surrounded-by-spaces
	// > The href content attribute, if specified, must contain a valid URL potentially surrounded by spaces.
	// > A string is a valid URL potentially surrounded by spaces if, after stripping leading and trailing ASCII whitespace from it, it is a valid URL string.
	// > ASCII whitespace is U+0009 TAB, U+000A LF, U+000C FF, U+000D CR, or U+0020 SPACE.
	//
	// We don't use strings.TrimSpace() here because it trim unicode spaces as well.
	base = strings.Trim(base, "\t\n\f\r ")

	baseURL, err := url.Parse(base)
	if err != nil {
		basetagLogger.Error("unable to parse base url", "error", err, "base", base)
		return
	}

	if baseURL.Scheme == "data" || baseURL.Scheme == "javascript" {
		basetagLogger.Error("the base url has the bad scheme", "base", base, "scheme", baseURL.Scheme)
		return
	}

	fallbackBaseURL := item.GetURL().GetParsed()
	newResolvedBaseURL := fallbackBaseURL.ResolveReference(baseURL)

	item.SetBase(newResolvedBaseURL)
}
