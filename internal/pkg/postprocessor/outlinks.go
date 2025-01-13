package postprocessor

import (
	"io"
	"strings"

	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/extractor"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/Zeno/pkg/models"
)

func extractOutlinks(item *models.Item) (outlinks []*models.URL, err error) {
	var (
		contentType = item.GetURL().GetResponse().Header.Get("Content-Type")
		logger      = log.NewFieldedLogger(&log.Fields{
			"component": "postprocessor.extractOutlinks",
		})
	)

	if item.GetURL().GetBody() == nil {
		logger.Error("no body to extract outlinks from", "url", item.GetURL().String(), "item", item.GetShortID())
		return
	}

	// Run specific extractors
	switch {
	case extractor.IsS3(item.GetURL()):
		outlinks, err = extractor.S3(item.GetURL())
		if err != nil {
			logger.Error("unable to extract outlinks", "err", err.Error(), "item", item.GetShortID())
			return outlinks, err
		}
	case extractor.IsSitemapXML(item.GetURL()):
		outlinks, err = extractor.XML(item.GetURL())
		if err != nil {
			logger.Error("unable to extract outlinks", "err", err.Error(), "item", item.GetShortID())
			return outlinks, err
		}
	case extractor.IsHTML(item.GetURL()):
		outlinks, err := extractor.HTMLOutlinks(item)
		if err != nil {
			logger.Error("unable to extract outlinks", "err", err.Error(), "item", item.GetShortID())
			return outlinks, err
		}
	default:
		logger.Debug("no extractor used for page", "content-type", contentType, "item", item.GetShortID())
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

	return outlinks, nil
}

func extractLinksFromPage(URL *models.URL) (links []*models.URL) {
	defer URL.RewindBody()

	// Extract links and dedupe them
	source, err := io.ReadAll(URL.GetBody())
	if err != nil {
		return links
	}

	rawLinks := utils.DedupeStrings(extractor.LinkRegexRelaxed.FindAllString(string(source), -1))

	// Validate links
	for _, link := range rawLinks {
		links = append(links, &models.URL{
			Raw:  link,
			Hops: URL.GetHops() + 1,
		})
	}

	return links
}
