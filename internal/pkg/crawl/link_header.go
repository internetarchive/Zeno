package crawl

import (
	"regexp"
	"strings"
)

// Represents a Link struct, containing a URL to which it links, and a Rel to define the relation
type Link struct {
	URL string
	Rel string
}

// Parse parses a raw Link header in the form:
//
//	<url1>; rel="what", <url2>; rel="any"; another="yes", <url3>; rel="thing"
//
// returning a slice of Link structs
// Each of these are separated by a `, ` and the in turn by a `; `, with the first always being the url, and the remaining the key-val pairs
// See: https://simon-frey.com/blog/link-header/, https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Link
func Parse(link string) []Link {
	var links []Link

	urlRegex := regexp.MustCompile(`^<(\S+)>$`)
	// var linkRegex = regexp.MustCompile(`^<(\S+)>(; (\S+)=(\S+))*$`)

	for _, link := range strings.Split(link, ", ") {
		parts := strings.Split(link, "; ")
		match := urlRegex.FindStringSubmatch(parts[0])
		url := match[1]
		rel := ""

		for _, attrs := range parts[1:] {
			key, value := ParseAttr(attrs)
			if key == "rel" {
				rel = value
				break
			}
		}
		links = append(links, Link{URL: url, Rel: rel})
	}

	return links
}

func ParseAttr(attrs string) (key, value string) {
	attrRegex := regexp.MustCompile(`^(\S+)=\"(\S+)\"$`)
	match := attrRegex.FindStringSubmatch(attrs)

	return match[1], match[2]
}
