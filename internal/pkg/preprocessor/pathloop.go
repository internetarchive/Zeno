package preprocessor

import (
	"strings"

	"github.com/internetarchive/Zeno/internal/pkg/config"
)

// maxRepetitions is the repetition threshold used to detect crawler traps.
// A URL is considered a trap when:
//   - any single path segment or query parameter key=value pair appears
//     more than maxRepetitions times, OR
//   - the path has 10+ segments and at least 2 distinct segments each
//     appear maxRepetitions or more times (the deep-path heuristic).
var maxRepetitions = config.Get().MaxSegmentReptition

// hasPathLoop checks if a URL contains repeating elements that indicate
// a crawler trap. It checks both:
// 1. Path segments (e.g. /a/b/a/b/a/b/...)
// 2. Query parameter key=value pairs (e.g. ?feature=applinks&feature=applinks&...)
//
// Returns true if any single path segment or query parameter pair appears
// more than maxRepetitions times, or if the path is deep (10+ segments)
// and at least 2 distinct segments each appear maxRepetitions or more times.
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

	// Deep-path heuristic: in paths with 10+ segments, flag when 2 or more
	// distinct segments each appear at least maxRepetitions times (>=, not >).
	// This catches complex traps where several segments repeat together
	// even if no single segment exceeds maxRepetitions.
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
