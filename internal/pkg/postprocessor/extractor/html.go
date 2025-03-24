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
	backgroundImageRegex = regexp.MustCompile(`(?:\(['"]?)(.*?)(?:['"]?\))`)
	urlRegex             = regexp.MustCompile(`(?m)url\((.*?)\)`)
)

func IsHTML(URL *models.URL) bool {
	return isContentType(URL.GetResponse().Header.Get("Content-Type"), "html") || strings.Contains(URL.GetMIMEType().String(), "html")
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

	// Define the valid link attributes to check
	validLinkAttributes := []string{
		"href",
		"data-href",
		"data-src",
		"data-srcset",
		"data-lazy-src",
		"src",
		"srcset",
	}

	// Match <a> tags with href, data-href, data-src, data-srcset, data-lazy-src, data-srcset, src, srcset
	if !slices.Contains(config.Get().DisableHTMLTag, "a") {
		document.Find("a").Each(func(index int, i *goquery.Selection) {
			for _, attr := range validLinkAttributes {
				link, exists := i.Attr(attr)
				if exists && link != "" {
					rawOutlinks = append(rawOutlinks, link)
				}
			}
		})
	}

	for _, rawOutlink := range rawOutlinks {
		resolvedURL, err := resolveURL(rawOutlink, item)
		if err != nil {
			logger.Debug("unable to resolve URL", "error", err, "url", item.GetURL().String(), "item", item.GetShortID())
			outlinks = append(outlinks, &models.URL{
				Raw: rawOutlink,
			})
		} else if resolvedURL != "" {
			normalizedURL, err := item.GetURL().NormalizeURL(resolvedURL)
			if err != nil {
				logger.Debug("unable to normalize URL", "error", err, "url", resolvedURL, "item", item.GetShortID())
				outlinks = append(outlinks, &models.URL{
					Raw: resolvedURL,
				})
			} else {
				outlinks = append(outlinks, &models.URL{
					Raw: normalizedURL,
				})
			}
		}
	}

	return outlinks, nil
}

func HTMLAssets(item *models.Item) (assets []*models.URL, err error) {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.extractor.HTMLAssets",
	})

	rawAssetsMap := make(map[string]bool)
	// Helper function to add to map instead of directly to slice
	addRawAsset := func(url string) {
		if url == "" {
			return
		}

		// Don't extract CSS elements that aren't URLs
		if strings.Contains(url, "%") ||
			strings.HasPrefix(url, "0.") ||
			strings.HasPrefix(url, "--font") ||
			strings.HasPrefix(url, "--size") ||
			strings.HasPrefix(url, "--color") ||
			strings.HasPrefix(url, "--shreddit") ||
			strings.HasPrefix(url, "100vh") ||
			strings.HasPrefix(url, "#wp-") {
			return
		}

		rawAssetsMap[url] = true
	}

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
				logger.Debug("unable to extract URLs from JSON in data-item attribute", "err", err, "url", item.GetURL().String(), "item", item.GetShortID())
			} else {
				for _, url := range URLsFromJSON {
					addRawAsset(url)
				}
			}
		}

		style, exists := i.Attr("style")
		if exists {
			matches := backgroundImageRegex.FindAllStringSubmatch(style, -1)

			for match := range matches {
				if len(matches[match]) > 0 {
					matchFound := matches[match][1]
					addRawAsset(matchFound)
				}
			}
		}

		dataPreview, exists := i.Attr("data-preview")
		if exists {
			if strings.HasPrefix(dataPreview, "http") {
				addRawAsset(dataPreview)
			}
		}
	})

	// Try to find assets in <a> tags
	if !slices.Contains(config.Get().DisableHTMLTag, "a") {
		var validAssetPath = []string{
			"static/", "assets/", "asset/", "images/", "image/", "img/",
		}

		var validAssetAttributes = []string{
			"href", "data-href", "data-src", "data-srcset",
			"data-lazy-src", "data-srcset", "src", "srcset",
		}

		document.Find("a").Each(func(index int, i *goquery.Selection) {
			for _, attr := range validAssetAttributes {
				link, exists := i.Attr(attr)
				if exists {
					if utils.StringContainsSliceElements(link, validAssetPath) {
						addRawAsset(link)
					}
				}
			}
		})
	}

	// Handle <img> tags
	if !slices.Contains(config.Get().DisableHTMLTag, "img") {
		document.Find("img").Each(func(index int, i *goquery.Selection) {
			// Handle various image attributes
			for _, attr := range []string{"src", "data-src", "data-lazy-src"} {
				if link, exists := i.Attr(attr); exists {
					addRawAsset(link)
				}
			}

			// Handle srcset and data-srcset attributes
			for _, srcsetAttr := range []string{"srcset", "data-srcset"} {
				if link, exists := i.Attr(srcsetAttr); exists {
					links := strings.Split(link, ",")
					for _, link := range links {
						addRawAsset(strings.Split(strings.TrimSpace(link), " ")[0])
					}
				}
			}
		})
	}

	// Handle video and audio tags
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
				addRawAsset(link)
			}
		})
	}

	// Handle style tags
	if !slices.Contains(config.Get().DisableHTMLTag, "style") {
		document.Find("style").Each(func(index int, i *goquery.Selection) {
			matches := urlRegex.FindAllStringSubmatch(i.Text(), -1)
			for match := range matches {
				matchReplacement := matches[match][1]
				matchReplacement = strings.Replace(matchReplacement, "'", "", -1)
				matchReplacement = strings.Replace(matchReplacement, "\"", "", -1)
				if !strings.Contains(matchReplacement, "http") {
					matchReplacement = strings.Replace(matchReplacement, "//", "http://", -1)
				}
				addRawAsset(matchReplacement)
			}
		})
	}

	// Handle script tags
	if !slices.Contains(config.Get().DisableHTMLTag, "script") {
		document.Find("script").Each(func(index int, i *goquery.Selection) {
			if link, exists := i.Attr("src"); exists {
				addRawAsset(link)
			}

			scriptType, exists := i.Attr("type")
			if exists && scriptType == "application/json" {
				URLsFromJSON, _, err := GetURLsFromJSON(json.NewDecoder(strings.NewReader(i.Text())))
				if err == nil {
					for _, url := range URLsFromJSON {
						addRawAsset(url)
					}
				}
			}

			// Apply regex on the script's HTML to extract potential assets
			outerHTML, err := goquery.OuterHtml(i)
			if err == nil {
				scriptLinks := utils.DedupeStrings(LinkRegexRelaxed.FindAllString(outerHTML, -1))
				for _, scriptLink := range scriptLinks {
					if strings.HasPrefix(scriptLink, "http") {
						// Escape URLs when unicode runes are present in the extracted URLs
						escapedLink, err := strconv.Unquote(`"` + scriptLink + `"`)
						if err == nil {
							addRawAsset(escapedLink)
						}
					}
				}
			}

			// Handle variable initialization in scripts
			if !strings.HasPrefix(i.Text(), "{") {
				assetsFromScriptContent, err := extractFromScriptContent(i.Text())
				if err == nil {
					for _, url := range assetsFromScriptContent {
						addRawAsset(url)
					}
				}
			}
		})
	}

	// Handle link tags
	if !slices.Contains(config.Get().DisableHTMLTag, "link") {
		document.Find("link").Each(func(index int, i *goquery.Selection) {
			if !config.Get().CaptureAlternatePages {
				relation, exists := i.Attr("rel")
				if exists && relation == "alternate" {
					return
				}
			}

			if link, exists := i.Attr("href"); exists {
				addRawAsset(link)
			}
		})
	}

	// Handle meta tags
	if !slices.Contains(config.Get().DisableHTMLTag, "meta") {
		document.Find("meta").Each(func(index int, i *goquery.Selection) {
			if link, exists := i.Attr("href"); exists {
				addRawAsset(link)
			}

			if link, exists := i.Attr("content"); exists && strings.Contains(link, "http") {
				addRawAsset(link)
			}
		})
	}

	// Handle source tags
	if !slices.Contains(config.Get().DisableHTMLTag, "source") {
		document.Find("source").Each(func(index int, i *goquery.Selection) {
			if link, exists := i.Attr("src"); exists {
				addRawAsset(link)
			}

			// Handle srcset and data-srcset attributes
			for _, srcsetAttr := range []string{"srcset", "data-srcset"} {
				if link, exists := i.Attr(srcsetAttr); exists {
					links := strings.Split(link, ",")
					for _, link := range links {
						addRawAsset(strings.Split(strings.TrimSpace(link), " ")[0])
					}
				}
			}
		})
	}

	var uniqueRawAssets []string
	for asset := range rawAssetsMap {
		uniqueRawAssets = append(uniqueRawAssets, asset)
	}
	// Create model URLs from unique raw assets
	for _, rawAsset := range uniqueRawAssets {
		normalizedURL, err := item.GetURL().NormalizeURL(rawAsset)
		if err != nil {
			normalizedURL = rawAsset
		}
		assets = append(assets, &models.URL{
			Raw: normalizedURL,
		})
	}

	return assets, nil
}
