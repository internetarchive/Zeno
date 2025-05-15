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
	case truthsocial.IsAccountURL(item.GetURL()):
		outlinks, err = truthsocial.GenerateAccountLookupURL(item.GetURL())
		if err != nil {
			logger.Error("unable to extract outlinks", "extractor", "truthsocial.GenerateAccountLookupURL", "err", err.Error(), "item", item.GetShortID(), "url", item.GetURL().String())
			return outlinks, err
		}
	case truthsocial.IsAccountLookupURL(item.GetURL()):
		outlinks, err = truthsocial.GenerateOutlinksURLsFromLookup(item.GetURL())
		if err != nil {
			logger.Error("unable to extract outlinks", "extractor", "truthsocial.GenerateOutlinksURLsFromLookup", "err", err.Error(), "item", item.GetShortID(), "url", item.GetURL().String())
			return outlinks, err
		}
	case extractor.IsObjectStorage(item.GetURL()):
		outlinks, err = extractor.ObjectStorage(item.GetURL())
		if err != nil {
			logger.Error("unable to extract outlinks from ObjectStorage", "extractor", "ObjectStorage", "err", err.Error(), "item", item.GetShortID(), "url", item.GetURL().String())
			return outlinks, err
		}
	case extractor.IsSitemapXML(item.GetURL()):
		var assets []*models.URL

		assets, outlinks, err = extractor.XML(item.GetURL())
		if err != nil {
			logger.Error("unable to extract outlinks", "extractor", "XML", "err", err.Error(), "item", item.GetShortID(), "url", item.GetURL().String())
			return outlinks, err
		}

		// Here we don't care about the difference between assets and outlinks,
		// we just want to extract all the URLs from the sitemap
		outlinks = append(outlinks, assets...)
	case extractor.IsHTML(item.GetURL()):
		outlinks, err = extractor.HTMLOutlinks(item)
		if err != nil {
			logger.Error("unable to extract outlinks", "extractor", "HTMLOutlinks", "err", err.Error(), "item", item.GetShortID(), "url", item.GetURL().String())
			return outlinks, err
		}
	case extractor.IsPDF(item.GetURL()):
		outlinks, err = extractor.PDF(item.GetURL())
		if err != nil {
			logger.Error("unable to extract outlinks", "extractor", "PDF", "err", err.Error(), "item", item.GetShortID(), "url", item.GetURL().String())
			return outlinks, err
		}
	case reddit.IsPostAPI(item.GetURL()):
		outlinks, err = reddit.ExtractAPIPostPermalinks(item)
		if err != nil {
			logger.Error("unable to extract outlinks", "extractor", "reddit.ExtractAPIPostPermalinks", "err", err.Error(), "item", item.GetShortID(), "url", item.GetURL().String())
			return outlinks, err
		}
	default:
		logger.Debug("no extractor used for page", "content-type", contentType, "item", item.GetShortID(), "url", item.GetURL().String())
		return outlinks, nil
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

	rawLinks := utils.DedupeStrings(extractor.LinkRegexStrict.FindAllString(string(source), -1))

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
