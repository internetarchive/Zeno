package extractor

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/pkg/models"
)

func IsHTML(URL *models.URL) bool {
	return isContentType(URL.GetResponse().Header.Get("Content-Type"), "html") || strings.Contains(URL.GetMIMEType().String(), "html")
}

func HTMLOutlinks(item *models.Item) (outlinks []*models.URL, err error) {
	defer item.GetURL().RewindBody()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.extractor.HTMLOutlinks",
	})

	itemURL := item.GetURL()
	if itemURL == nil {
		logger.Error("item has no URL object, cannot extract outlinks", "item_id", item.GetShortID())
		return nil, fmt.Errorf("item has no URL object")
	}

	itemParsedURL := itemURL.GetParsed()
	if itemParsedURL == nil {
		logger.Error("item's URL object has no parsed URL, cannot extract outlinks", "item_id", item.GetShortID(), "item_raw_url", itemURL.Raw)
		return nil, fmt.Errorf("item URL's parsed form is nil, cannot extract outlinks")
	}

	// Retrieve (potentially creates it) the document from the body
	document, err := itemURL.GetDocument()
	if err != nil {
		return nil, err
	}

	// Extract the base tag if it exists
	extractBaseTag(item, document)

	rawOutlinks := ExtractOutlinksFromDocument(document, item.GetBase(), config.Get())

	extractedResolvedURLsMap := make(map[string]bool)

	for _, rawOutlink := range rawOutlinks {
		resolvedURLString, resolveErr := resolveRawURLString(rawOutlink, item, logger)

		if resolveErr != nil {
			logger.Error("critical error resolving raw outlink", "raw_url", rawOutlink, "error", resolveErr, "item", item.GetShortID())
			continue
		}

		if resolvedURLString == "" {
			continue
		}

		itemMainURLString := itemParsedURL.String()

		if resolvedURLString == item.GetBase() || resolvedURLString == itemMainURLString {
			logger.Debug("discarding outlink because it is the same as the base URL or current URL after resolution", "resolved_url", resolvedURLString, "item", item.GetShortID())
			continue
		}

		if extractedResolvedURLsMap[resolvedURLString] {
			logger.Debug("discarding duplicate resolved outlink", "resolved_url", resolvedURLString, "item", item.GetShortID())
			continue
		}
		extractedResolvedURLsMap[resolvedURLString] = true

		resolvedURLParsed, parseErr := url.Parse(resolvedURLString)
		if parseErr != nil {
			logger.Error("failed to parse resolved outlink URL string for scheme check", "resolved_url", resolvedURLString, "error", parseErr, "item", item.GetShortID())
			continue
		}

		if !resolvedURLParsed.IsAbs() || (resolvedURLParsed.Scheme != "http" && resolvedURLParsed.Scheme != "https") {
			logger.Debug("discarding non-http/s or non-absolute outlink", "resolved_url", resolvedURLString, "item", item.GetShortID())
			continue
		}


		newOutlinkURL := &models.URL{
			Raw: resolvedURLString, 
		}

		err = newOutlinkURL.Parse()
		if err != nil {
			logger.Error("failed to parse resolved outlink string for models.URL object", "resolved_url", resolvedURLString, "error", err, "item", item.GetShortID())
			continue
		}

		outlinks = append(outlinks, newOutlinkURL)
	}

	return outlinks, nil
}

func resolveRawURLString(rawURL string, item *models.Item, logger *log.FieldedLogger) (string, error) {
	if item == nil {
		return "", fmt.Errorf("cannot resolve URL, item is nil")
	}
	itemURL := item.GetURL()
	if itemURL == nil {
		return "", fmt.Errorf("cannot resolve URL, item URL is nil")
	}

	itemParsedURL := itemURL.GetParsed()
	if itemParsedURL == nil {
		return "", fmt.Errorf("item URL's parsed form is unexpectedly nil in helper")
	}

	var baseURL *url.URL
	itemBase := item.GetBase()
	if itemBase != "" {
		parsedBaseFromTag, parseErr := url.Parse(itemBase)
		if parseErr != nil {
			logger.Warn("invalid base URL string from item's GetBase(), falling back to item's main URL for resolution", "base", itemBase, "item_url", itemURL.Raw, "item_id", item.GetShortID())
			baseURL = itemParsedURL
		} else {
			baseURL = parsedBaseFromTag
		}
	} else {
		baseURL = itemParsedURL
	}

	parsedRawURL, err := url.Parse(rawURL)
	if err != nil {
		logger.Debug("invalid raw URL string found, cannot parse for resolution", "raw_url", rawURL, "error", err, "item_id", item.GetShortID())
		return "", nil
	}

	resolvedURL := baseURL.ResolveReference(parsedRawURL)

	return resolvedURL.String(), nil
}


func HTMLAssets(item *models.Item) (assets []*models.URL, err error) {
	defer item.GetURL().RewindBody()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.extractor.HTMLAssets",
	})

	itemURL := item.GetURL()
	if itemURL == nil {
		logger.Error("item has no URL object, cannot extract assets", "item_id", item.GetShortID())
		return nil, fmt.Errorf("item has no URL object")
	}

	itemParsedURL := itemURL.GetParsed()
	if itemParsedURL == nil {
		logger.Error("item's URL object has no parsed URL, cannot extract assets", "item_id", item.GetShortID(), "item_raw_url", itemURL.Raw)
		return nil, fmt.Errorf("item URL's parsed form is nil, cannot extract assets")
	}

	document, err := itemURL.GetDocument()
	if err != nil {
		logger.Debug("unable to get document from item URL", "error", err, "item", item.GetShortID())
		return nil, err
	}

	// Extract the base tag if it exists
	extractBaseTag(item, document)
	rawAssets := ExtractAssetsFromDocument(document, item.GetBase(), config.Get())
	extractedResolvedURLsMap := make(map[string]bool)

	for _, rawAsset := range rawAssets {
		resolvedAssetString, resolveErr := resolveRawURLString(rawAsset, item, logger)

		if resolveErr != nil {
			logger.Error("critical error resolving raw asset link", "raw_url", rawAsset, "error", resolveErr, "item", item.GetShortID())
			continue
		}

		if resolvedAssetString == "" {
			continue
		}

		if extractedResolvedURLsMap[resolvedAssetString] {
			logger.Debug("discarding duplicate resolved asset", "resolved_url", resolvedAssetString, "item", item.GetShortID())
			continue
		}
		extractedResolvedURLsMap[resolvedAssetString] = true

		newAssetURL := &models.URL{
			Raw: resolvedAssetString, 
		}
		err = newAssetURL.Parse()
		if err != nil {
			logger.Error("failed to parse resolved asset string for models.URL object", "resolved_url", resolvedAssetString, "error", err, "item", item.GetShortID())
			continue
		}

		assets = append(assets, newAssetURL)
	}

	return assets, nil
}