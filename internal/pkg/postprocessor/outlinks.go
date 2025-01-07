package postprocessor

import (
	"io"
	"strings"

	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/extractor"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/Zeno/pkg/models"
)

func extractOutlinks(URL *models.URL, item *models.Item) (outlinks []*models.URL, err error) {
	var (
		contentType = URL.GetResponse().Header.Get("Content-Type")
		logger      = log.NewFieldedLogger(&log.Fields{
			"component": "postprocessor.extractOutlinks",
		})
	)

	// Run specific extractors
	switch {
	case extractor.IsS3(URL):
		outlinks, err = extractor.S3(URL)
		if err != nil {
			logger.Error("unable to extract outlinks", "err", err.Error(), "item", item.GetShortID())
			return outlinks, err
		}
	default:
		logger.Debug("no extractor used for page", "content-type", contentType, "item", item.GetShortID())
	}

	// If the page is a text/* content type, extract links from the body (aggressively)
	if strings.Contains(contentType, "text/") {
		outlinks = append(outlinks, extractLinksFromPage(URL)...)
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
