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
	LinkRegex       = regexp.MustCompile(`(?i)https?://[^<>'",\s/]+\.[^<>'",\s/]+(?:/[^<>'",\s]*)?`) // Adapted from heritrix3's UriUtils (Apache License 2.0)
	quotedLinkRegex = regexp.MustCompile(`['"](https?://[^'"]+)['"]`)
	AssetsRegex     = `(?i)\b(?:src|href)=["']([^"']+\.(?:css|js|png|jpg|jpeg|gif|svg|webp|woff|woff2|ttf|eot))["']`
)

// Helper function to call FindAllStringSubmatch on quotedLinkRegex and return only the capturing group (Quoted URL).
func QuotedLinkRegexFindAll(s string) []string {
	matches := quotedLinkRegex.FindAllStringSubmatch(s, -1)
	result := make([]string, 0, len(matches))
	for i := range matches {
		if len(matches[i]) > 1 {
			result = append(result, matches[i][1])
		}
	}
	return result
}

// hasFileExtension checks if a URL has a file extension in it.
// It might yield false positives, like https://example.com/super.idea,
// but it's good enough for our purposes.
func hasFileExtension(s string) bool {
	// Remove query and fragment portion (?...) (#...)
	if i := strings.IndexAny(s, `?#`); i != -1 {
		s = s[:i]
	}

	// Exclude URLs that only contain a protocol and domain
	if (strings.HasPrefix(s, "//") || strings.Contains(s, "://")) && strings.Count(s, "/") == 2 {
		return false // e.g., "//example.com", "http://example.com"
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
