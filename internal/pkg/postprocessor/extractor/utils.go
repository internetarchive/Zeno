package extractor

import (
	"regexp"
	"sort"
	"strings"

	"github.com/internetarchive/Zeno/pkg/models"
	"mvdan.cc/xurls/v2"
)

var (
	LinkRegexRelaxed = xurls.Relaxed()
	LinkRegexStrict  = xurls.Strict()
	LinkRegex        = regexp.MustCompile(`['"]((http|https)://[^'"]+)['"]`)
	AssetsRegex      = `(?i)\b(?:src|href)=["']([^"']+\.(?:css|js|png|jpg|jpeg|gif|svg|webp|woff|woff2|ttf|eot))["']`
)

func isContentType(header, targetContentType string) bool {
	// Lowercase the header and target content type for case-insensitive comparison
	header = strings.ToLower(header)
	targetContentType = strings.ToLower(targetContentType)

	return strings.Contains(header, targetContentType)
}

// sortURLs sorts a slice of *url.URL
func sortURLs(urls []*models.URL) {
	sort.Slice(urls, func(i, j int) bool {
		return urls[i].Raw < urls[j].Raw
	})
}
