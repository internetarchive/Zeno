package extractor

import (
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/CorentinB/warc/pkg/spooledtempfile"
	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/Zeno/pkg/models"
)

var (
	backgroundImageRegex = regexp.MustCompile(`(?:\(['"]?)(.*?)(?:['"]?\))`)
	urlRegex             = regexp.MustCompile(`(?m)url\((.*?)\)`)
)

func IsHTML(URL *models.URL) bool {
	return isContentType(URL.GetResponse().Header.Get("Content-Type"), "html") || strings.Contains(URL.GetMIMEType().String(), "html")
}

func GetDocument(body spooledtempfile.ReadSeekCloser) (doc *goquery.Document, err error) {
	document, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, err
	}
	return document, nil
}

func HTMLOutlinks(item *models.Item) (outlinks []*models.URL, err error) {
	defer item.GetURL().RewindBody()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.extractor.HTMLOutlinks",
	})

	var rawOutlinks []string

	// Retrieve (potentially creates it) the document from the body
	document, err := GetDocument(item.GetURL().GetBody())
	item.GetURL().RewindBody()
	if err != nil {
		return nil, err
	}

	// Extract the base tag if it exists
	extractBaseTag(item, document)

	// Match <a> tags with href, data-href, data-src, data-srcset, data-lazy-src, data-srcset, src, srcset
	if !slices.Contains(config.Get().DisableHTMLTag, "a") {
		document.Find("a").Each(func(index int, i *goquery.Selection) {
			for _, node := range i.Nodes {
				for _, attr := range node.Attr {
					link := attr.Val
					rawOutlinks = append(rawOutlinks, link)
				}
			}
		})
	}

	for _, rawOutlink := range rawOutlinks {
		resolvedURL, err := resolveURL(rawOutlink, item)
		if err != nil {
			logger.Debug("unable to resolve URL", "error", err, "url", item.GetURL().String(), "item", item.GetShortID())
		} else if resolvedURL != "" {
			outlinks = append(outlinks, &models.URL{
				Raw: resolvedURL,
			})
			continue
		}

		outlinks = append(outlinks, &models.URL{
			Raw: rawOutlink,
		})
	}
	// Make sure the goquery document's memory can be freed
	document = nil

	return outlinks, nil
}

func HTMLAssets(item *models.Item) (assets []*models.URL, err error) {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.extractor.HTMLAssets",
	})

	var rawAssets []string

	// Retrieve (potentially creates it) the document from the body
	document, err := GetDocument(item.GetURL().GetBody())
	item.GetURL().RewindBody()
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
				logger.Debug("unable to extract URLs from JSON in data-item attribute", "err", err, "url", item.GetURL().String(), "item", item.GetShortID())
			} else {
				rawAssets = append(rawAssets, URLsFromJSON...)
			}
		}

		style, exists := i.Attr("style")
		if exists {
			matches := backgroundImageRegex.FindAllStringSubmatch(style, -1)

			for match := range matches {
				if len(matches[match]) > 0 {
					matchFound := matches[match][1]

					// Don't extract CSS elements that aren't URLs
					if strings.Contains(matchFound, "%") ||
						strings.HasPrefix(matchFound, "0.") ||
						strings.HasPrefix(matchFound, "--font") ||
						strings.HasPrefix(matchFound, "--size") ||
						strings.HasPrefix(matchFound, "--color") ||
						strings.HasPrefix(matchFound, "--shreddit") ||
						strings.HasPrefix(matchFound, "100vh") {
						continue
					}

					rawAssets = append(rawAssets, matchFound)
				}
			}
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
			matches := urlRegex.FindAllStringSubmatch(i.Text(), -1)
			for match := range matches {
				matchReplacement := matches[match][1]
				matchReplacement = strings.Replace(matchReplacement, "'", "", -1)
				matchReplacement = strings.Replace(matchReplacement, "\"", "", -1)

				// If the URL already has http (or https), we don't need add anything to it.
				if !strings.Contains(matchReplacement, "http") {
					matchReplacement = strings.Replace(matchReplacement, "//", "http://", -1)
				}

				if strings.HasPrefix(matchReplacement, "#wp-") {
					continue
				}

				rawAssets = append(rawAssets, matchReplacement)
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
				if scriptType == "application/json" {
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
				logger.Debug("unable to extract outer HTML from script tag", "err", err, "url", item.GetURL().String(), "item", item.GetShortID())
			} else {
				scriptLinks := utils.DedupeStrings(LinkRegexRelaxed.FindAllString(outerHTML, -1))
				for _, scriptLink := range scriptLinks {
					if strings.HasPrefix(scriptLink, "http") {
						// Escape URLs when unicode runes are present in the extracted URLs
						scriptLink, err := strconv.Unquote(`"` + scriptLink + `"`)
						if err != nil {
							logger.Debug("unable to escape URL from JSON in script tag", "error", err, "url", item.GetURL().String(), "item", item.GetShortID())
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
					logger.Debug("unable to extract URLs from JSON in script tag", "error", err, "url", item.GetURL().String(), "item", item.GetShortID())
				} else {
					rawAssets = append(rawAssets, assetsFromScriptContent...)
				}
			}
		})
	}

	if !slices.Contains(config.Get().DisableHTMLTag, "link") {
		document.Find("link").Each(func(index int, i *goquery.Selection) {
			if !config.Get().CaptureAlternatePages {
				relation, exists := i.Attr("rel")
				if exists && relation == "alternate" {
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
		document.Find("meta").Each(func(index int, i *goquery.Selection) {
			link, exists := i.Attr("href")
			if exists {
				rawAssets = append(rawAssets, link)
			}
			link, exists = i.Attr("content")
			if exists {
				if strings.Contains(link, "http") {
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

	for _, rawAsset := range rawAssets {
		assets = append(assets, &models.URL{
			Raw: rawAsset,
		})

	}
	// Make sure the goquery document's memory can be freed
	document = nil

	return assets, nil
}
