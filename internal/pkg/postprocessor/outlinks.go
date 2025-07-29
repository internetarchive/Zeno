package postprocessor

import (
	"io"
	"strings"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/domainscrawl"
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
	linksFromLinkHeader := extractor.ExtractURLsFromHeader(item.GetURL())
	if linksFromLinkHeader != nil {
		outlinks = append(outlinks, linksFromLinkHeader...)
	}

	// If the page is a text/* content type, extract links from the body (aggressively)
	if strings.Contains(contentType, "text/") {
		outlinks = append(outlinks, extractLinksFromPage(item.GetURL())...)
	}

	outlinks = filterURLsByProtocol(outlinks)

	// Set the hops level to the item's level + 1
	for _, outlink := range outlinks {
		outlink.SetHops(item.GetURL().GetHops() + 1)
	}

	return outlinks, nil
}

func extractLinksFromPage(URL *models.URL) (links []*models.URL) {
	defer URL.RewindBody()

	// Extract links and dedupe them
	source, err := io.ReadAll(URL.GetBody())
	if err != nil {
		return links
	}
	var rawLinks []string
	if !config.Get().StrictRegex {
		rawLinks = utils.DedupeStrings(extractor.LinkRegex.FindAllString(string(source), -1))
	} else {
		rawLinks = utils.DedupeStrings(extractor.LinkRegexStrict.FindAllString(string(source), -1))
	}
	// Validate links
	for _, link := range rawLinks {
		links = append(links, &models.URL{
			Raw:  link,
			Hops: URL.GetHops() + 1,
		})
	}

	return links
}

func shouldExtractOutlinks(item *models.Item) bool {
	// Bypass the hop count if we are domain crawling to ensure we don't miss an outlink from a domain we are interested in
	if domainscrawl.Enabled() && item.GetURL().GetBody() != nil {
		return true
	}

	// Match pure hops count
	if item.GetURL().GetHops() < config.Get().MaxHops && item.GetURL().GetBody() != nil {
		return true
	}

	return false
}
