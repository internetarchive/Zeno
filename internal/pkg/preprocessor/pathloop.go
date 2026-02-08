package preprocessor

import "strings"

// maxRepetitions is the maximum number of times a single path segment or
// query parameter key=value pair can appear in a URL before it is considered
// a crawler trap.
const maxRepetitions = 3

// hasPathLoop checks if a URL contains repeating elements that indicate
// a crawler trap. It checks both:
// 1. Path segments (e.g. /a/b/a/b/a/b/...)
// 2. Query parameter key=value pairs (e.g. ?feature=applinks&feature=applinks&...)
//
// Returns true if any single path segment or query parameter pair appears
// more than maxRepetitions times.
//
// path is the URL path component (from Pathname()).
// search is the URL query string (from Search()).
func hasPathLoop(path, search string) bool {
	// Check path segments
	segments := strings.Split(path, "/")
	counts := make(map[string]int, len(segments))
	nonEmptySegments := 0
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		nonEmptySegments++
		counts[seg]++
		if counts[seg] > maxRepetitions {
			return true
		}
	}

	// In deep paths (10+ segments), flag when multiple different segments each
	// appear at least maxRepetitions times — this indicates a complex crawler
	// trap where several segments repeat together.
	if nonEmptySegments >= 10 {
		segmentsAtThreshold := 0
		for _, count := range counts {
			if count >= maxRepetitions {
				segmentsAtThreshold++
				if segmentsAtThreshold >= 2 {
					return true
				}
			}
		}
	}

	// Check query parameter key=value pairs
	query := strings.TrimPrefix(search, "?")
	if query != "" {
		params := strings.Split(query, "&")
		paramCounts := make(map[string]int, len(params))
		for _, param := range params {
			if param == "" {
				continue
			}
			paramCounts[param]++
			if paramCounts[param] > maxRepetitions {
				return true
			}
		}
	}

	return false
}
