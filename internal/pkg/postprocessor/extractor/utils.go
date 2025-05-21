package extractor

import (
	"regexp"
	"sort"
	"strings"

	"github.com/internetarchive/Zeno/pkg/models"
	"mvdan.cc/xurls/v2"
)

var (
	LinkRegexStrict = xurls.Strict()
	LinkRegex       = regexp.MustCompile(`['"]((http|https)://[^'"]+)['"]`)
	AssetsRegex     = `(?i)\b(?:src|href)=["']([^"']+\.(?:css|js|png|jpg|jpeg|gif|svg|webp|woff|woff2|ttf|eot))["']`
)

// hasFileExtension checks if a URL has a file extension in it.
// It might yield false positives, like https://example.com/super.idea,
// but it's good enough for our purposes.
func hasFileExtension(s string) bool {
	// Remove fragment portion (#...)
	if i := strings.IndexByte(s, '#'); i != -1 {
		s = s[:i]
	}
	// Remove query portion (?...)
	if i := strings.IndexByte(s, '?'); i != -1 {
		s = s[:i]
	}

	// Keep only the substring after the last slash
	if slashPos := strings.LastIndexByte(s, '/'); slashPos != -1 {
		s = s[slashPos+1:]
	}

	// Find the last '.' in the file name
	dotPos := strings.LastIndexByte(s, '.')
	if dotPos == -1 || dotPos == len(s)-1 {
		// No '.' or '.' is the last character -> no valid extension
		return false
	}

	return true
}

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

// Convert from []string -> []*models.URL
func toURLs(s []string) []*models.URL {
	outlinks := make([]*models.URL, 0, len(s))
	for _, link := range s {
		outlinks = append(outlinks, &models.URL{Raw: link})
	}
	return outlinks
}
