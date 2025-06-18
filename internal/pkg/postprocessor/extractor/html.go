package extractor

import (
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/Zeno/pkg/models"
)

var (
	onclickRegex = regexp.MustCompile(`window\.location(?:\.href)?\s*=\s*['"]([^'"]+)['"]`)
)

func IsHTML(URL *models.URL) bool {
	return URL.GetMIMEType() != nil && strings.Contains(URL.GetMIMEType().String(), "html")
}

func HTMLOutlinks(item *models.Item) (outlinks []*models.URL, err error) {
	defer item.GetURL().RewindBody()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.extractor.HTMLOutlinks",
	})

	var rawOutlinks []string

	// Retrieve (potentially creates it) the document from the body
	document, err := item.GetURL().GetDocument()
	if err != nil {
		return nil, err
	}

	// Extract the base tag if it exists
	extractBaseTag(item, document)

	// Match <a> tags with href, data-href, data-src, data-srcset, data-lazy-src, data-srcset, src, srcset
	// Extract potential URLs from <a> tags using common attributes
	if !slices.Contains(config.Get().DisableHTMLTag, "a") {
		attrs := []string{
			"href",
			"data-href",
			"data-url",
			"data-link",
			"data-redirect-url",
			"ping",
			"onclick",
			"ondblclick",
			"router-link",
			"to",
		}

		document.Find("a").Each(func(index int, sel *goquery.Selection) {
			for _, key := range attrs {
				val, exists := sel.Attr(key)
				if !exists || val == "" {
					continue
				}

				if key == "onclick" || key == "ondblclick" {
					// Attempt to extract URL from JS like window.location = '...';
					if matches := onclickRegex.FindStringSubmatch(val); len(matches) > 1 {
						rawOutlinks = append(rawOutlinks, matches[1])
					}
					continue
				}

				rawOutlinks = append(rawOutlinks, val)
			}
		})
	}

	if !slices.Contains(config.Get().DisableHTMLTag, "iframe") {
		document.Find("iframe[src]").Each(func(index int, i *goquery.Selection) {
			if src, exists := i.Attr("src"); exists && src != "" {
				rawOutlinks = append(rawOutlinks, src)
			}
		})
	}

	if !slices.Contains(config.Get().DisableHTMLTag, "area") {
		document.Find("area[href]").Each(func(index int, i *goquery.Selection) {
			if href, exists := i.Attr("href"); exists && href != "" {
				rawOutlinks = append(rawOutlinks, href)
			}
		})
	}

	for _, rawOutlink := range rawOutlinks {
		resolvedURL, err := resolveURL(rawOutlink, item)
		if err != nil {
			logger.Debug("unable to resolve URL", "error", err, "url", item.GetURL(), "item", item.GetShortID())
		} else if resolvedURL != "" {
			outlinks = append(outlinks, &models.URL{
				Raw: resolvedURL,
			})
			continue
		}

		// Discard URLs that are the same as the base URL or the current URL
		if (item.GetBase() != nil && rawOutlink == item.GetBase().String()) || rawOutlink == item.GetURL().String() {
			logger.Debug("discarding outlink because it is the same as the base URL or current URL", "url", rawOutlink, "item", item.GetShortID())
			continue
		}

		outlinks = append(outlinks, &models.URL{
			Raw: rawOutlink,
		})
	}

	return outlinks, nil
}

func HTMLAssets(item *models.Item) (assets []*models.URL, err error) {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.extractor.HTMLAssets",
	})

	var rawAssets []string

	// Retrieve (potentially creates it) the document from the body
	document, err := item.GetURL().GetDocument()
	if err != nil {
		return nil, err
	}

	// Extract the base tag if it exists
	extractBaseTag(item, document)

	// Get assets from JSON payloads in data-item values
	// Check all elements style attributes for background-image & also data-preview
	document.Find("[data-item], [style], [data-preview]").Each(func(index int, i *goquery.Selection) {
		dataItem, exists := i.Attr("data-item")
		if exists {
			URLsFromJSON, _, err := GetURLsFromJSON(json.NewDecoder(strings.NewReader(dataItem)))
			if err != nil {
				logger.Debug("unable to extract URLs from JSON in data-item attribute", "err", err, "url", item.GetURL(), "item", item.GetShortID())
			} else {
				rawAssets = append(rawAssets, URLsFromJSON...)
			}
		}

		style, exists := i.Attr("style")
		if exists {
			links, _, err := ExtractFromStringCSS(style, true)
			if err != nil {
				cssLogger.Warn("error parsing inline attribute style CSS", "err", err, "url", item.GetURL(), "item", item.GetShortID(), "links", len(links))
			}
			rawAssets = append(rawAssets, links...)
		}

		dataPreview, exists := i.Attr("data-preview")
		if exists {
			if strings.HasPrefix(dataPreview, "http") {
				rawAssets = append(rawAssets, dataPreview)
			}
		}
	})

	// Try to find assets in <a> tags.. this is a bit funky
	if !slices.Contains(config.Get().DisableHTMLTag, "a") {
		var validAssetPath = []string{
			"static/",
			"assets/",
			"asset/",
			"images/",
			"image/",
			"img/",
		}

		var validAssetAttributes = []string{
			"href",
			"data-href",
			"data-src",
			"data-srcset",
			"data-lazy-src",
			"data-srcset",
			"src",
			"srcset",
		}

		document.Find("a").Each(func(index int, i *goquery.Selection) {
			for _, attr := range validAssetAttributes {
				link, exists := i.Attr(attr)
				if exists {
					if utils.StringContainsSliceElements(link, validAssetPath) {
						rawAssets = append(rawAssets, link)
					}
				}
			}
		})
	}

	// Extract assets on the page (images, scripts, videos..)
	if !slices.Contains(config.Get().DisableHTMLTag, "img") {
		document.Find("img").Each(func(index int, i *goquery.Selection) {
			link, exists := i.Attr("src")
			if exists {
				rawAssets = append(rawAssets, link)
			}

			link, exists = i.Attr("data-src")
			if exists {
				rawAssets = append(rawAssets, link)
			}

			link, exists = i.Attr("data-lazy-src")
			if exists {
				rawAssets = append(rawAssets, link)
			}

			link, exists = i.Attr("data-srcset")
			if exists {
				links := strings.Split(link, ",")
				for _, link := range links {
					rawAssets = append(rawAssets, strings.Split(strings.TrimSpace(link), " ")[0])
				}
			}

			link, exists = i.Attr("srcset")
			if exists {
				links := strings.Split(link, ",")
				for _, link := range links {
					rawAssets = append(rawAssets, strings.Split(strings.TrimSpace(link), " ")[0])
				}
			}
		})
	}

	var targetElements = []string{}
	if !slices.Contains(config.Get().DisableHTMLTag, "video") {
		targetElements = append(targetElements, "video[src]")
	}
	if !slices.Contains(config.Get().DisableHTMLTag, "audio") {
		targetElements = append(targetElements, "audio[src]")
	}
	if len(targetElements) > 0 {
		document.Find(strings.Join(targetElements, ", ")).Each(func(index int, i *goquery.Selection) {
			if link, exists := i.Attr("src"); exists {
				rawAssets = append(rawAssets, link)
			}
		})
	}

	if !slices.Contains(config.Get().DisableHTMLTag, "style") {
		document.Find("style").Each(func(index int, i *goquery.Selection) {
			links, atImportLinks, err := ExtractFromStringCSS(i.Text(), false)
			if err != nil {
				cssLogger.Warn("error parsing HTML style block CSS", "err", err, "url", item.GetURL(), "item", item.GetShortID(), "links", len(links), "at_import_links", len(atImportLinks))
			}
			for _, link := range links {
				// If the URL already has http (or https), we don't need add anything to it.
				if !strings.Contains(link, "http") {
					link = strings.Replace(link, "//", "http://", -1)
				}

				if strings.HasPrefix(link, "#wp-") {
					continue
				}

				rawAssets = append(rawAssets, link)
			}
		})
	}

	if !slices.Contains(config.Get().DisableHTMLTag, "script") {
		document.Find("script").Each(func(index int, i *goquery.Selection) {
			link, exists := i.Attr("src")
			if exists {
				rawAssets = append(rawAssets, link)
			}

			scriptType, exists := i.Attr("type")
			if exists {
				if strings.Contains(scriptType, "json") {
					URLsFromJSON, _, err := GetURLsFromJSON(json.NewDecoder(strings.NewReader(i.Text())))
					if err != nil {
						// TODO: maybe add back when https://github.com/internetarchive/Zeno/issues/147 is fixed
						// c.Log.Debug("unable to extract URLs from JSON in script tag", "error", err, "url", URL)
					} else {
						rawAssets = append(rawAssets, URLsFromJSON...)
					}
				}
			}

			// Apply regex on the script's HTML to extract potential assets
			outerHTML, err := goquery.OuterHtml(i)
			if err != nil {
				logger.Debug("unable to extract outer HTML from script tag", "err", err, "url", item.GetURL(), "item", item.GetShortID())
			} else {
				var scriptLinks []string
				if !config.Get().StrictRegex {
					scriptLinks = utils.DedupeStrings(LinkRegex.FindAllString(outerHTML, -1))
				} else {
					scriptLinks = utils.DedupeStrings(LinkRegexStrict.FindAllString(outerHTML, -1))
				}
				for _, scriptLink := range scriptLinks {
					if strings.HasPrefix(scriptLink, "http") {
						// Escape URLs when unicode runes are present in the extracted URLs
						scriptLink, err := strconv.Unquote(`"` + scriptLink + `"`)
						if err != nil {
							logger.Debug("unable to escape URL from JSON in script tag", "error", err, "url", item.GetURL(), "item", item.GetShortID())
							continue
						}
						rawAssets = append(rawAssets, scriptLink)
					}
				}
			}

			// Some <script> embed variable initialisation, we can strip the variable part and just scrape JSON
			if !strings.HasPrefix(i.Text(), "{") {
				assetsFromScriptContent, err := extractFromScriptContent(i.Text())
				if err != nil {
					logger.Debug("unable to extract URLs from JSON in script tag", "error", err, "url", item.GetURL(), "item", item.GetShortID())
				} else {
					rawAssets = append(rawAssets, assetsFromScriptContent...)
				}
			}
		})
	}

	if !slices.Contains(config.Get().DisableHTMLTag, "link") {
		document.Find("link[href]").Each(func(index int, i *goquery.Selection) {
			if !config.Get().CaptureAlternatePages {
				if i.AttrOr("rel", "") == "alternate" {
					return
				}
			}
			link, exists := i.Attr("href")
			if exists {
				rawAssets = append(rawAssets, link)
			}
		})
	}

	if !slices.Contains(config.Get().DisableHTMLTag, "meta") {
		document.Find("meta[href], meta[content]").Each(func(index int, i *goquery.Selection) {
			link, exists := i.Attr("href")
			if exists {
				rawAssets = append(rawAssets, link)
			}
			content, exists := i.Attr("content")
			if exists {
				link, exists := extractURLFromContent(content)
				if exists {
					rawAssets = append(rawAssets, link)
				}
			}
		})
	}

	if !slices.Contains(config.Get().DisableHTMLTag, "source") {
		document.Find("source").Each(func(index int, i *goquery.Selection) {
			link, exists := i.Attr("src")
			if exists {
				rawAssets = append(rawAssets, link)
			}

			link, exists = i.Attr("srcset")
			if exists {
				links := strings.Split(link, ",")
				for _, link := range links {
					rawAssets = append(rawAssets, strings.Split(strings.TrimSpace(link), " ")[0])
				}
			}

			link, exists = i.Attr("data-srcset")
			if exists {
				links := strings.Split(link, ",")
				for _, link := range links {
					rawAssets = append(rawAssets, strings.Split(strings.TrimSpace(link), " ")[0])
				}
			}
		})
	}

	// Extract WACZ files from replayweb embeds (docs: https://replayweb.page/docs/embedding/ )
	if !slices.Contains(config.Get().DisableHTMLTag, "replay-web-page") {
		document.Find("replay-web-page[source]").Each(func(index int, i *goquery.Selection) {
			source, exists := i.Attr("source")
			if exists {
				rawAssets = append(rawAssets, source)
			}
		})
	}

	for _, rawAsset := range rawAssets {
		resolvedURL, err := resolveURL(rawAsset, item)
		if err != nil {
			var baseURL string
			if item.GetBase() != nil {
				baseURL = item.GetBase().String()
			}
			logger.Debug("unable to resolve URL", "error", err, "item_url", item.GetURL(), "base_url", baseURL, "target", rawAsset, "item", item.GetShortID())
		} else if resolvedURL != "" {
			assets = append(assets, &models.URL{
				Raw: resolvedURL,
			})
			continue
		}

		assets = append(assets, &models.URL{
			Raw: rawAsset,
		})

	}

	return assets, nil
}

var contentURLRegex = regexp.MustCompile(`(?i)\burl\s*=\s*(\S+)`)

// Must support: "0; url=https://refr1.com", "http://other.com" and be case insensitive
func extractURLFromContent(content string) (string, bool) {
	matches := contentURLRegex.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.Trim(matches[1], `"'`), true
	}
	if LinkRegexStrict.MatchString(content) {
		return content, true
	}
	return "", false
}
