// Package domainscrawl is a postprocessing component that parse domains from a given input and stores them for later matching.
// It can store naive domains, full URLs, and regex patterns. It can then check if a given URL matches any of the stored patterns.
package domainscrawl

import (
	"net/url"
	"regexp"
	"strings"
	"sync"
)

type matchEngine struct {
	sync.RWMutex
	regexes []*regexp.Regexp
	domains []string
	urls    []url.URL
}

var (
	globalMatcher = &matchEngine{
		regexes: make([]*regexp.Regexp, 0),
		domains: make([]string, 0),
		urls:    make([]url.URL, 0),
	}
)

// Reset the matcher to its initial state
func Reset() {
	globalMatcher.Lock()
	defer globalMatcher.Unlock()

	globalMatcher.regexes = make([]*regexp.Regexp, 0)
	globalMatcher.domains = make([]string, 0)
	globalMatcher.urls = make([]url.URL, 0)
}

// AddElements takes a slice of strings, heuristically determines their type, and stores them
func AddElements(elements []string) error {
	globalMatcher.Lock()
	defer globalMatcher.Unlock()

	for _, element := range elements {
		// Try to parse as a URL first
		parsedURL, err := url.Parse(element)
		if err == nil && parsedURL.Scheme != "" && parsedURL.Host != "" {
			// If it has a scheme and host, it's a full URL
			globalMatcher.urls = append(globalMatcher.urls, *parsedURL)
			continue
		}

		// Check if it's a naive domain (e.g., "example.com")
		if isNaiveDomain(element) {
			globalMatcher.domains = append(globalMatcher.domains, element)
			continue
		}

		// Otherwise, assume it's a regex
		re, err := regexp.Compile(element)
		if err != nil {
			return err
		}
		globalMatcher.regexes = append(globalMatcher.regexes, re)
	}
	return nil
}

// Match checks if a given URL matches any of the stored patterns
func Match(u *url.URL) bool {
	globalMatcher.RLock()
	defer globalMatcher.RUnlock()

	// Check against naive domains
	for _, domain := range globalMatcher.domains {
		if isSubdomainOrExactMatch(u.Host, domain) {
			return true
		}
	}

	// Check against full URLs
	for _, storedURL := range globalMatcher.urls {
		if storedURL.String() == u.String() {
			return true
		}
		// If the stored URL has no query, path, or fragment, we greedily match (sub)domain
		if storedURL.RawQuery == "" && storedURL.Path == "" && storedURL.Fragment == "" && isSubdomainOrExactMatch(u.Host, storedURL.Host) {
			return true
		}
	}

	// Check against regex patterns
	for _, re := range globalMatcher.regexes {
		if re.MatchString(u.String()) {
			return true
		}
	}

	return false
}

// Check if a string is a naive domain (e.g., "example.com")
func isNaiveDomain(s string) bool {
	// A naive domain should not contain a scheme, path, or query
	if strings.Contains(s, "://") || strings.Contains(s, "/") || strings.Contains(s, "?") || strings.Contains(s, "#") {
		return false
	}
	// Check if it has a dot and no spaces
	return strings.Contains(s, ".") && !strings.Contains(s, " ")
}

// isSubdomainOrExactMatch checks if the given host is a subdomain or an exact match of the domain
func isSubdomainOrExactMatch(host, domain string) bool {
	// Exact match
	if host == domain {
		return true
	}

	// Subdomain match (e.g., "sub.example.com" matches "example.com")
	if strings.HasSuffix(host, "."+domain) {
		return true
	}

	return false
}
