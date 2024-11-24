package postprocessor

import (
	"net/url"

	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/extractor"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
)

func extractLinksFromText(source string) (links []*url.URL) {
	// Extract links and dedupe them
	rawLinks := utils.DedupeStrings(extractor.LinkRegexRelaxed.FindAllString(source, -1))

	// Validate links
	for _, link := range rawLinks {
		URL, err := url.Parse(link)
		if err != nil {
			continue
		}

		links = append(links, URL)
	}

	return links
}
