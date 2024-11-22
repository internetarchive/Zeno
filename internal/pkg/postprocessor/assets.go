package postprocessor

import (
	"bytes"
	"io"
	"regexp"
	"strings"

	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/extractor"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/Zeno/pkg/models"
)

func extractAssets(URL *models.URL, item *models.Item) (err error) {
	var (
		assets      []*models.URL
		body        = bytes.NewBuffer(nil)
		contentType = URL.GetResponse().Header.Get("Content-Type")
		logger      = log.NewFieldedLogger(&log.Fields{
			"component": "postprocessor.extractAssets",
		})
	)

	// Read the body in a bytes buffer, then put a copy of it in the URL's response body
	_, err = io.Copy(body, URL.GetResponse().Body)
	if err != nil {
		logger.Error("unable to read response body", "err", err.Error(), "item", item.GetShortID())
		return
	}

	URL.GetResponse().Body = io.NopCloser(bytes.NewReader(body.Bytes()))

	// Extract assets from the body using the appropriate extractor
	switch {
	case strings.Contains(contentType, "html"):
		assets, err = extractor.HTML(URL, item)
		if err != nil {
			logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID())
			return err
		}
	default:
		logger.Debug("no extractor found for content type", "content-type", contentType, "item", item.GetShortID())
	}

	// Extract URLs from the body using regex
	var URLs []string
	for _, regex := range []*regexp.Regexp{extractor.LinkRegexStrict, extractor.LinkRegex} {
		URLs = append(URLs, regex.FindAllString(body.String(), -1)...)
	}

	for _, URL := range utils.DedupeStrings(URLs) {
		assets = append(assets, &models.URL{
			Raw:  URL,
			Hops: item.URL.GetHops(),
		})
	}

	for _, asset := range assets {
		// If the item has a value of 0 for ChildsCaptured, it means that we are  on the first iteration
		// of the postprocessor and we allow another iteration to capture the assets of assets
		if item.GetChildsCaptured() == 0 {
			item.AddChild(asset)
		}
	}

	return
}
