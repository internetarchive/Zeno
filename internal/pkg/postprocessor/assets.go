package postprocessor

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/extractor"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/Zeno/pkg/models"
	"mvdan.cc/xurls/v2"
)

var (
	backgroundImageRegex = regexp.MustCompile(`(?:\(['"]?)(.*?)(?:['"]?\))`)
	urlRegex             = regexp.MustCompile(`(?m)url\((.*?)\)`)
	linkRegex            = xurls.Relaxed()
)

func extractAssets(seed *models.Item) (err error) {
	var rawAssets []string

	// Build goquery doc from response
	doc, err := goquery.NewDocumentFromReader(seed.GetURL().GetResponse().Body)
	if err != nil {
		return err
	}

	// Get assets from JSON payloads in data-item values
	doc.Find("[data-item]").Each(func(index int, item *goquery.Selection) {
		dataItem, exists := item.Attr("data-item")
		if exists {
			URLsFromJSON, err := extractor.GetURLsFromJSON([]byte(dataItem))
			if err != nil {
				logger.Debug("unable to extract URLs from JSON in data-item attribute", "err", err, "url", seed.GetURL(), "item", seed.GetShortID())
			} else {
				rawAssets = append(rawAssets, URLsFromJSON...)
			}
		}
	})

	// Check all elements style attributes for background-image & also data-preview
	doc.Find("*").Each(func(index int, item *goquery.Selection) {
		style, exists := item.Attr("style")
		if exists {
			matches := backgroundImageRegex.FindAllStringSubmatch(style, -1)

			for match := range matches {
				if len(matches[match]) > 0 {
					matchFound := matches[match][1]

					// Don't extract CSS elements that aren't URLs
					if strings.Contains(matchFound, "%") || strings.HasPrefix(matchFound, "0.") || strings.HasPrefix(matchFound, "--font") || strings.HasPrefix(matchFound, "--size") || strings.HasPrefix(matchFound, "--color") || strings.HasPrefix(matchFound, "--shreddit") || strings.HasPrefix(matchFound, "100vh") {
						continue
					}

					rawAssets = append(rawAssets, matchFound)
				}
			}
		}

		dataPreview, exists := item.Attr("data-preview")
		if exists {
			if strings.HasPrefix(dataPreview, "http") {
				rawAssets = append(rawAssets, dataPreview)
			}
		}
	})

	// Extract assets on the page (images, scripts, videos..)
	if !utils.StringInSlice("img", config.Get().DisableHTMLTag) {
		doc.Find("img").Each(func(index int, item *goquery.Selection) {
			link, exists := item.Attr("src")
			if exists {
				rawAssets = append(rawAssets, link)
			}

			link, exists = item.Attr("data-src")
			if exists {
				rawAssets = append(rawAssets, link)
			}

			link, exists = item.Attr("data-lazy-src")
			if exists {
				rawAssets = append(rawAssets, link)
			}

			link, exists = item.Attr("data-srcset")
			if exists {
				links := strings.Split(link, ",")
				for _, link := range links {
					rawAssets = append(rawAssets, strings.Split(strings.TrimSpace(link), " ")[0])
				}
			}

			link, exists = item.Attr("srcset")
			if exists {
				links := strings.Split(link, ",")
				for _, link := range links {
					rawAssets = append(rawAssets, strings.Split(strings.TrimSpace(link), " ")[0])
				}
			}
		})
	}

	if !utils.StringInSlice("video", config.Get().DisableHTMLTag) {
		doc.Find("video").Each(func(index int, item *goquery.Selection) {
			link, exists := item.Attr("src")
			if exists {
				rawAssets = append(rawAssets, link)
			}
		})
	}

	if !utils.StringInSlice("style", config.Get().DisableHTMLTag) {
		doc.Find("style").Each(func(index int, item *goquery.Selection) {
			matches := urlRegex.FindAllStringSubmatch(item.Text(), -1)
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

	if !utils.StringInSlice("script", config.Get().DisableHTMLTag) {
		doc.Find("script").Each(func(index int, item *goquery.Selection) {
			link, exists := item.Attr("src")
			if exists {
				rawAssets = append(rawAssets, link)
			}

			scriptType, exists := item.Attr("type")
			if exists {
				if scriptType == "application/json" {
					URLsFromJSON, err := extractor.GetURLsFromJSON([]byte(item.Text()))
					if err != nil {
						// TODO: maybe add back when https://github.com/internetarchive/Zeno/issues/147 is fixed
						// c.Log.Debug("unable to extract URLs from JSON in script tag", "error", err, "url", URL)
					} else {
						rawAssets = append(rawAssets, URLsFromJSON...)
					}
				}
			}

			// Apply regex on the script's HTML to extract potential assets
			outerHTML, err := goquery.OuterHtml(item)
			if err != nil {
				logger.Debug("unable to extract outer HTML from script tag", "err", err, "url", seed.GetURL(), "item", seed.GetShortID())
			} else {
				scriptLinks := utils.DedupeStrings(linkRegex.FindAllString(outerHTML, -1))
				for _, scriptLink := range scriptLinks {
					if strings.HasPrefix(scriptLink, "http") {
						// Escape URLs when unicode runes are present in the extracted URLs
						scriptLink, err := strconv.Unquote(`"` + scriptLink + `"`)
						if err != nil {
							logger.Debug("unable to escape URL from JSON in script tag", "error", err, "url", scriptLink, "item", seed.GetShortID())
							continue
						}
						rawAssets = append(rawAssets, scriptLink)
					}
				}
			}

			// Some <script> embed variable initialisation, we can strip the variable part and just scrape JSON
			if !strings.HasPrefix(item.Text(), "{") {
				jsonContent := strings.SplitAfterN(item.Text(), "=", 2)

				if len(jsonContent) > 1 {
					var (
						openSeagullCount   int
						closedSeagullCount int
						payloadEndPosition int
					)

					// figure out the end of the payload
					for pos, char := range jsonContent[1] {
						if char == '{' {
							openSeagullCount++
						} else if char == '}' {
							closedSeagullCount++
						} else {
							continue
						}

						if openSeagullCount > 0 {
							if openSeagullCount == closedSeagullCount {
								payloadEndPosition = pos
								break
							}
						}
					}

					if len(jsonContent[1]) > payloadEndPosition {
						URLsFromJSON, err := extractor.GetURLsFromJSON([]byte(jsonContent[1][:payloadEndPosition+1]))
						if err != nil {
							// TODO: maybe add back when https://github.com/internetarchive/Zeno/issues/147 is fixed
							// c.Log.Debug("unable to extract URLs from JSON in script tag", "error", err, "url", URL)
						} else {
							rawAssets = append(rawAssets, URLsFromJSON...)
						}
					}
				}
			}
		})
	}

	if !utils.StringInSlice("link", config.Get().DisableHTMLTag) {
		doc.Find("link").Each(func(index int, item *goquery.Selection) {
			if !config.Get().CaptureAlternatePages {
				relation, exists := item.Attr("rel")
				if exists && relation == "alternate" {
					return
				}
			}

			link, exists := item.Attr("href")
			if exists {
				rawAssets = append(rawAssets, link)
			}
		})
	}

	if !utils.StringInSlice("audio", config.Get().DisableHTMLTag) {
		doc.Find("audio").Each(func(index int, item *goquery.Selection) {
			link, exists := item.Attr("src")
			if exists {
				rawAssets = append(rawAssets, link)
			}
		})
	}

	if !utils.StringInSlice("meta", config.Get().DisableHTMLTag) {
		doc.Find("meta").Each(func(index int, item *goquery.Selection) {
			link, exists := item.Attr("href")
			if exists {
				rawAssets = append(rawAssets, link)
			}
			link, exists = item.Attr("content")
			if exists {
				if strings.Contains(link, "http") {
					rawAssets = append(rawAssets, link)
				}
			}
		})
	}

	if !utils.StringInSlice("source", config.Get().DisableHTMLTag) {
		doc.Find("source").Each(func(index int, item *goquery.Selection) {
			link, exists := item.Attr("src")
			if exists {
				rawAssets = append(rawAssets, link)
			}

			link, exists = item.Attr("srcset")
			if exists {
				links := strings.Split(link, ",")
				for _, link := range links {
					rawAssets = append(rawAssets, strings.Split(strings.TrimSpace(link), " ")[0])
				}
			}

			link, exists = item.Attr("data-srcset")
			if exists {
				links := strings.Split(link, ",")
				for _, link := range links {
					rawAssets = append(rawAssets, strings.Split(strings.TrimSpace(link), " ")[0])
				}
			}
		})
	}

	for _, rawAsset := range rawAssets {
		seed.AddChild(&models.URL{
			Raw:  rawAsset,
			Hops: seed.GetURL().GetHops(),
		})
	}

	return nil
}
