// Package domainscrawl is a postprocessing component that parse domains from a given input and stores them for later matching.
// It can store naive domains, full URLs, and regex patterns. It can then check if a given URL matches any of the stored patterns.
package domainscrawl

import (
	"bufio"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/ImVexed/fasturl"
)

type matchEngine struct {
	sync.RWMutex
	enabled bool
	regexes []*regexp.Regexp
	domains map[string]struct{}
	urls    []url.URL
}

var (
	globalMatcher = &matchEngine{
		enabled: false,
		regexes: make([]*regexp.Regexp, 0),
		domains: make(map[string]struct{}),
		urls:    make([]url.URL, 0),
	}
)

// Reset the matcher to its initial state
func Reset() {
	globalMatcher.Lock()
	defer globalMatcher.Unlock()

	globalMatcher.enabled = false
	globalMatcher.regexes = make([]*regexp.Regexp, 0)
	globalMatcher.domains = make(map[string]struct{})
	globalMatcher.urls = make([]url.URL, 0)
}

// Enabled returns true if the domainscrawl matcher is enabled
func Enabled() bool {
	globalMatcher.RLock()
	defer globalMatcher.RUnlock()

	return globalMatcher.enabled
}

// AddElements takes a slice of strings or files containing patterns, heuristically determines their type, and stores them
func AddElements(elements []string, files []string) error {
	globalMatcher.Lock()
	defer globalMatcher.Unlock()

	globalMatcher.enabled = true

	for _, file := range files {
		file, err := os.Open(file)
		if err != nil {
			return err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			elements = append(elements, scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			return err
		}
	}

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
			globalMatcher.domains[element] = struct{}{}
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
func Match(rawURL string) bool {
	u, err := fasturl.ParseURL(rawURL)
	if err != nil {
		return false
	}

	globalMatcher.RLock()
	defer globalMatcher.RUnlock()

	// Check against naive domains using map for O(1) lookup
	if _, exists := globalMatcher.domains[u.Host]; exists {
		return true
	}

	// Check for subdomains
	for domain := range globalMatcher.domains {
		if len(domain) <= len(u.Host) && isSubdomain(u.Host, domain) {
			return true
		}
	}

	// Check against full URLs
	for _, storedURL := range globalMatcher.urls {
		if storedURL.String() == rawURL {
			return true
		}

		// If the stored URL has no query, path, or fragment, we greedily match (sub)domain
		if storedURL.RawQuery == "" && storedURL.Path == "" && storedURL.Fragment == "" && isSubdomain(u.Host, storedURL.Host) {
			return true
		}
	}

	// Check against regex patterns
	for _, re := range globalMatcher.regexes {
		if re.MatchString(rawURL) {
			return true
		}
	}

	return false
}

// Check if a string is a naive domain (e.g., "example.com")
func isNaiveDomain(s string) bool {
	// A naive domain should not contain a scheme, path, or query
	if strings.Contains(s, "://") || strings.ContainsAny(s, "/?#") {
		return false
	}
	// Check if it has a dot and no spaces
	return strings.Contains(s, ".") && !strings.Contains(s, " ")
}

// isSubdomain checks if the given host is a subdomain or an exact match of the domain
func isSubdomain(host, domain string) bool {
	return host == domain || (len(host) > len(domain) && strings.HasSuffix(host, "."+domain))
}
