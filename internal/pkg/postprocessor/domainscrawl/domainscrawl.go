// Package domainscrawl is a postprocessing component that parse domains from a given input and stores them for later matching.
// It can store naive domains, full URLs, and regex patterns. It can then check if a given URL matches any of the stored patterns.
package domainscrawl

import (
	"bufio"
	"log/slog"
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
	domains *ART
	urls    []url.URL
}

var (
	globalMatcher = &matchEngine{
		enabled: false,
		regexes: make([]*regexp.Regexp, 0),
		domains: newART(),
		urls:    make([]url.URL, 0),
	}
)

// Reset the matcher to its initial state
func Reset() {
	globalMatcher.Reset()
}

// Enabled returns true if the domainscrawl matcher is enabled
func Enabled() bool {
	return globalMatcher.Enabled()
}

// AddElements takes a slice of strings or files containing patterns, heuristically determines their type, and stores them
func AddElements(elements []string, files []string) error {
	return globalMatcher.AddElements(elements, files)
}

// Match checks if a given URL matches any of the stored patterns
func Match(rawURL string) bool {
	return globalMatcher.Match(rawURL)
}

// NewMatcher creates a new matchEngine instance for testing or isolated usage
func NewMatcher() *matchEngine {
	return &matchEngine{
		enabled: false,
		regexes: make([]*regexp.Regexp, 0),
		domains: newART(),
		urls:    make([]url.URL, 0),
	}
}

// Reset resets the matcher to its initial state
func (m *matchEngine) Reset() {
	m.Lock()
	defer m.Unlock()

	m.enabled = false
	m.regexes = make([]*regexp.Regexp, 0)
	m.domains = newART()
	m.urls = make([]url.URL, 0)
}

// Enabled returns true if the domainscrawl matcher is enabled
func (m *matchEngine) Enabled() bool {
	m.RLock()
	defer m.RUnlock()

	return m.enabled
}

// AddElements takes a slice of strings or files containing patterns, heuristically determines their type, and stores them
func (m *matchEngine) AddElements(elements []string, files []string) error {
	m.Lock()
	defer m.Unlock()

	m.enabled = true

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)

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
			m.urls = append(m.urls, *parsedURL)
			continue
		}

		// Check if it's a naive domain (e.g., "example.com")
		if isNaiveDomain(element) {
			m.domains.Insert(element)
			continue
		}

		// Otherwise, assume it's a regex
		re, err := regexp.Compile(element)
		if err != nil {
			return err
		}
		m.regexes = append(m.regexes, re)
	}

	slog.Info("domainscrawl", "enabled", m.enabled, "domains", m.domains.Size(), "urls", len(m.urls), "regexes", len(m.regexes))

	return nil
}

// Match checks if a given URL matches any of the stored patterns
func (m *matchEngine) Match(rawURL string) bool {
	u, err := fasturl.ParseURL(rawURL)
	if err != nil {
		return false
	}

	m.RLock()
	defer m.RUnlock()

	// Check against naive domains, trying an exact match (O(1) lookup, fastest), else do a prefix search for subdomains (O(n) where n is the length of the domain)
	if m.domains.ExactMatch(u.Host) || m.domains.PrefixMatch(u.Host) {
		return true
	}

	// Check against full URLs
	for _, storedURL := range m.urls {
		if storedURL.String() == rawURL {
			return true
		}

		// If the stored URL has no query, path, or fragment, we greedily match (sub)domain
		if storedURL.RawQuery == "" && storedURL.Path == "" && storedURL.Fragment == "" && isSubdomain(u.Host, storedURL.Host) {
			return true
		}
	}

	// Check against regex patterns
	for _, re := range m.regexes {
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
