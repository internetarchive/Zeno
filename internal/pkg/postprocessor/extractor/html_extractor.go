package extractor

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
)

var (
	cssURLRegex = regexp.MustCompile(`url\(['"]?([^'"]+)['"]?\)`)
)

func ExtractOutlinksFromDocument(doc *goquery.Document, baseURL string, cfg *config.Config) []string {
	var rawOutlinks []string
	extractedURLs := make(map[string]bool)

	// Match <a> tags with href, data-href, data-src, data-srcset, data-lazy-src, data-srcset, src, srcset
	// Extract potential URLs from <a> tags using common attributes
	linkAttrs := []string{
		"href",
		"data-href",
		"data-url",
		"data-link",
		"data-redirect-url",
		"ping",
		"router-link",
		"to",
	}

	disableATag := false
	if cfg != nil {
		disableATag = slices.Contains(cfg.DisableHTMLTag, "a")
	}

	if !disableATag {
		doc.Find("a").Each(func(index int, sel *goquery.Selection) {
			for _, key := range linkAttrs {
				val, exists := sel.Attr(key)
				if !exists || val == "" {
					continue
				}

				if !extractedURLs[val] {
					rawOutlinks = append(rawOutlinks, val)
					extractedURLs[val] = true
				}

				if key == "onclick" {
					re := regexp.MustCompile(`window\.location(?:\.href)?\s*=\s*['"]([^'"]+)['"]`)
					if matches := re.FindStringSubmatch(val); len(matches) > 1 {
						onclickURL := matches[1]
						if !extractedURLs[onclickURL] {
							rawOutlinks = append(rawOutlinks, onclickURL)
							extractedURLs[onclickURL] = true
						}
					}
				}
			}
		})
	}

	if cfg != nil && !slices.Contains(cfg.DisableHTMLTag, "script") {
		doc.Find("script").Each(func(index int, sel *goquery.Selection) {
			scriptContent := sel.Text()
			scriptLinks := utils.DedupeStrings(LinkRegexStrict.FindAllString(scriptContent, -1))
			for _, scriptLink := range scriptLinks {
				rawURL := strings.Trim(scriptLink, `'"`)
				if !extractedURLs[rawURL] {
					rawOutlinks = append(rawOutlinks, rawURL)
					extractedURLs[rawURL] = true
				}
			}
		})
	}

	return rawOutlinks
}

func ExtractAssetsFromDocument(doc *goquery.Document, baseURL string, cfg *config.Config) []string {
	var rawAssets []string
	extractedURLs := make(map[string]bool)

	addRawURL := func(rawURL string) {
		if rawURL != "" && !extractedURLs[rawURL] {
			rawAssets = append(rawAssets, rawURL)
			extractedURLs[rawURL] = true
		}
	}

	assetTags := map[string][]string{
		"img":    {"src", "data-src", "data-lazy-src"},
		"video":  {"src"},
		"audio":  {"src"},
		"source": {"src"},
		"script": {"src"},
		"link":   {"href"},
		"meta":   {"content", "href"},
	}

	isDisabled := func(tag string) bool {
		if cfg == nil {
			return false
		}
		return slices.Contains(cfg.DisableHTMLTag, tag)
	}

	for tag, attrs := range assetTags {
		if isDisabled(tag) {
			continue
		}
		doc.Find(tag).Each(func(index int, sel *goquery.Selection) {
			for _, attr := range attrs {
				if val, exists := sel.Attr(attr); exists {
					if tag == "meta" && attr == "content" {
						if strings.HasPrefix(val, "http") || strings.HasPrefix(val, "/") || strings.HasPrefix(val, "./") || strings.HasPrefix(val, "../") {
							addRawURL(val)
						}
					} else {
						addRawURL(val)
					}
				}
			}
		})
	}

	srcsetTags := []string{"img", "source"}
	for _, tag := range srcsetTags {
		if isDisabled(tag) {
			continue
		}
		doc.Find(tag).Each(func(index int, sel *goquery.Selection) {
			srcsetAttrs := []string{"srcset", "data-srcset"}
			for _, attr := range srcsetAttrs {
				if val, exists := sel.Attr(attr); exists {
					links := strings.Split(val, ",")
					for _, link := range links {
						urlPart := strings.Split(strings.TrimSpace(link), " ")[0]
						addRawURL(urlPart)
					}
				}
			}
		})
	}

	doc.Find("[style]").Each(func(index int, sel *goquery.Selection) {
		if style, exists := sel.Attr("style"); exists {
			matches := cssURLRegex.FindAllStringSubmatch(style, -1)
			for _, match := range matches {
				if len(match) > 1 {
					url := match[1]
					
					// Don't extract CSS elements that aren't URLs
					if !(strings.Contains(url, "%") ||
						strings.HasPrefix(url, "0.") ||
						strings.HasPrefix(url, "--font") ||
						strings.HasPrefix(url, "--size") ||
						strings.HasPrefix(url, "--color") ||
						strings.HasPrefix(url, "--shreddit") ||
						strings.HasPrefix(url, "100vh")) {
						addRawURL(url)
					}
				}
			}
		}
	})

	if !isDisabled("style") {
		doc.Find("style").Each(func(index int, sel *goquery.Selection) {
			matches := cssURLRegex.FindAllStringSubmatch(sel.Text(), -1)
			for _, match := range matches {
				if len(match) > 1 {
					url := match[1]
					addRawURL(url)
				}
			}
		})
	}

	if !isDisabled("script") {
		doc.Find("script").Each(func(index int, sel *goquery.Selection) {
			scriptContent := sel.Text()

			scriptType, exists := sel.Attr("type")
			if exists && strings.Contains(scriptType, "json") {
				URLsFromJSON, _, err := GetURLsFromJSON(json.NewDecoder(strings.NewReader(scriptContent)))
				if err == nil {
					for _, u := range URLsFromJSON {
						addRawURL(u)
					}
				}
			} else {
				scriptLinks := utils.DedupeStrings(LinkRegexStrict.FindAllString(scriptContent, -1))
				for _, scriptLink := range scriptLinks {
					rawURL := strings.Trim(scriptLink, `'"`)
					addRawURL(rawURL)
				}

				assetsFromScriptContent, err := extractFromScriptContent(scriptContent)
				if err == nil {
					for _, u := range assetsFromScriptContent {
						addRawURL(u)
					}
				}
			}
		})
	}


	// Get assets from JSON payloads in data-item values
	// Check all elements style attributes for background-image & also data-preview
	doc.Find("[data-item], [data-preview]").Each(func(index int, sel *goquery.Selection) {
		if dataItem, exists := sel.Attr("data-item"); exists {
			URLsFromJSON, _, err := GetURLsFromJSON(json.NewDecoder(strings.NewReader(dataItem)))
			if err == nil {
				for _, u := range URLsFromJSON {
					addRawURL(u)
				}
			}
		}
		if dataPreview, exists := sel.Attr("data-preview"); exists {
			addRawURL(dataPreview)
		}
	})

	if !isDisabled("a") {
		var validAssetPath = []string{
			"static/", "assets/", "asset/", "images/", "image/", "img/", "css/", "style/", "font/", "fonts/", "script/", "scripts/", "js/",
		}
		assetLinkAttrs := []string{"href", "data-href", "data-src", "data-srcset", "data-lazy-src", "src", "srcset"}

		doc.Find("a").Each(func(index int, sel *goquery.Selection) {
			for _, attr := range assetLinkAttrs {
				if link, exists := sel.Attr(attr); exists {
					if utils.StringContainsSliceElements(link, validAssetPath) {
						addRawURL(link)
					}
				}
			}
		})
	}

	return rawAssets
}
