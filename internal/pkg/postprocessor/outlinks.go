package postprocessor

import (
	"bufio"
	"strings"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/extractor"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/sitespecific/reddit"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/sitespecific/truthsocial"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/Zeno/pkg/models"
)

type OutlinkExtractor interface {
	Support(extractor.Mode) bool // Support checks if the extractor supports the given mode
	Match(*models.URL) bool
	Extract(*models.URL) ([]*models.URL, error)
}

var outlinkExtractors = []OutlinkExtractor{
	truthsocial.TruthsocialAccountOutlinkExtractor{},
	truthsocial.TruthsocialAccountLookupOutlinkExtractor{},
	extractor.ObjectStorageOutlinkExtractor{},
	extractor.SitemapXMLOutlinkExtractor{},
	extractor.HTMLOutlinkExtractor{},
	extractor.PDFOutlinkExtractor{},
	reddit.RedditPostAPIOutlinkExtractor{},
}

func extractOutlinks(item *models.Item) (outlinks []*models.URL, err error) {
	var (
		contentType string
		logger      = log.NewFieldedLogger(&log.Fields{
			"component": "postprocessor.extractOutlinks",
		})
	)

	if item.GetURL().GetResponse() != nil {
		contentType = item.GetURL().GetResponse().Header.Get("Content-Type")
	} else {
		contentType = "text/html" // Headless, hardcoded to HTML
	}

	if item.GetURL().GetBody() == nil {
		logger.Error("no body to extract outlinks from", "url", item.GetURL(), "item", item.GetShortID())
		return
	}

	mode := extractor.ModeGeneral
	if config.Get().Headless {
		mode = extractor.ModeHeadless
	}

	// Run specific extractors
	for _, p := range outlinkExtractors {
		if !p.Support(mode) {
			continue
		}

		if p.Match(item.GetURL()) {
			outlinks, err = p.Extract(item.GetURL())
			break
		}
	}

	if outlinks == nil && err == nil {
		logger.Debug("no extractor used for page", "content-type", contentType, "item", item.GetShortID(), "url", item.GetURL())
	}

	// Try to extract links from link headers
	linkHeaderExtractor := extractor.LinkHeaderExtractor{}
	if linkHeaderExtractor.Support(mode) && linkHeaderExtractor.Match(item.GetURL()) {
		linksFromLinkHeader := linkHeaderExtractor.ExtractLink(item.GetURL())
		outlinks = append(outlinks, linksFromLinkHeader...)
	}

	// If the page is a text/* content type, extract links from the body (aggressively)
	if strings.Contains(contentType, "text/") {
		outlinks = append(outlinks, extractLinksFromPage(item.GetURL())...)
	}

	outlinks = filterURLsByProtocol(outlinks)
	outlinks = filterMaxOutlinks(outlinks)

	// Set the hops level to the item's level + 1
	for _, outlink := range outlinks {
		outlink.SetHops(item.GetURL().GetHops() + 1)
	}

	return outlinks, nil
}

const minLinkLength = len("http://a.b")

func extractLinksFromPage(URL *models.URL) (links []*models.URL) {
	defer URL.RewindBody()

	// Extract links and dedupe them
	scanner := bufio.NewScanner(URL.GetBody())
	scanner.Split(bufio.ScanWords)

	var rawLinks []string

	for scanner.Scan() {
		token := scanner.Text()

		var tokenLinks []string

		if !config.Get().StrictRegex {
			// perf optimization: skip short token and those without a protocol
			if len(token) < minLinkLength || !strings.Contains(token, "://") {
				continue
			}

			tokenLinks = extractor.LinkRegex.FindAllString(token, -1)
		} else {
			tokenLinks = extractor.LinkRegexStrict.FindAllString(token, -1)
		}

		rawLinks = append(rawLinks, tokenLinks...)
	}

	if err := scanner.Err(); err != nil {
		logger.Error("error scanning body for links", "err", err.Error(), "url", URL.String())
	}

	rawLinks = utils.DedupeStrings(rawLinks)

	// Validate links
	for _, link := range rawLinks {
		links = append(links, &models.URL{
			Raw: link,
		})
	}

	return links
}

func filterMaxOutlinks(outlinks []*models.URL) []*models.URL {
	limit := config.Get().MaxOutlinks
	outlinksLen := len(outlinks)
	if limit > 0 && outlinksLen > 0 && outlinksLen > limit {
		return outlinks[:limit]
	}
	return outlinks
}
