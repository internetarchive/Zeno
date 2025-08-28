package domainscrawl

import (
	"testing"
)

// Test isNaiveDomain function
func TestIsNaiveDomain(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"example.com", true},
		{"sub.example.com", true},
		{"example.com/path", false},
		{"https://example.com", false},
		{"example.com?query=1", false},
		{"example.com#fragment", false},
		{"https://example.org/path?query=1", false},
		{"example", false},     // No dot
		{"example com", false}, // Contains space
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isNaiveDomain(tt.input)
			if result != tt.expected {
				t.Errorf("isNaiveDomain(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

// Test isSubdomain function
func TestIsSubdomain(t *testing.T) {
	tests := []struct {
		host     string
		domain   string
		expected bool
	}{
		{"sub.example.com", "example.com", true},  // Subdomain match
		{"example.com", "sub.example.com", false}, // Not a subdomain
		{"example.org", "example.com", false},     // Different domain
	}

	for _, tt := range tests {
		t.Run(tt.host+"_"+tt.domain, func(t *testing.T) {
			result := isSubdomain(tt.host, tt.domain)
			if result != tt.expected {
				t.Errorf("isSubdomain(%q, %q) = %v, expected %v", tt.host, tt.domain, result, tt.expected)
			}
		})
	}
}

// Test Enabled function
func TestEnabled(t *testing.T) {
	Reset()
	if Enabled() {
		t.Error("Enabled() = true, expected false")
	}

	err := AddElements([]string{"example.com"}, nil)
	if err != nil {
		t.Fatalf("Failed to add elements: %v", err)
	}

	if !Enabled() {
		t.Error("Enabled() = false, expected true")
	}
}

// Test AddElements function
func TestAddElements(t *testing.T) {
	tests := []struct {
		name               string
		elements           []string
		expectErr          bool
		expectNaiveDomains []string
		expectURLs         []string
		expectRegexes      []string
	}{
		{
			name:               "Valid naive domain",
			elements:           []string{"example.com"},
			expectErr:          false,
			expectNaiveDomains: []string{"example.com"},
			expectURLs:         nil,
			expectRegexes:      nil,
		},
		{
			name:               "Valid full URL",
			elements:           []string{"https://example.org/path?query=1"},
			expectErr:          false,
			expectNaiveDomains: nil,
			expectURLs:         []string{"https://example.org/path?query=1"},
			expectRegexes:      nil,
		},
		{
			name:               "Valid regex",
			elements:           []string{`^https?://(www\.)?example\.net/.*`},
			expectErr:          false,
			expectURLs:         nil,
			expectRegexes:      []string{`^https?://(www\.)?example\.net/.*`},
			expectNaiveDomains: nil,
		},
		{
			name:               "Invalid regex",
			elements:           []string{`[invalid`},
			expectErr:          true,
			expectURLs:         nil,
			expectRegexes:      nil,
			expectNaiveDomains: nil,
		},
		{
			name:               "Mixed valid and invalid",
			elements:           []string{"example.com", `[invalid`},
			expectErr:          true,
			expectURLs:         nil,
			expectRegexes:      nil,
			expectNaiveDomains: []string{"example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Reset()
			err := AddElements(tt.elements, nil)
			if (err != nil) != tt.expectErr {
				t.Errorf("AddElements() error = %v, expectErr = %v", err, tt.expectErr)
			}

			// Check naive domains - convert map to slice for comparison
			domainMap := globalMatcher.domains
			domainSlice := domainMap.Range()

			if len(tt.expectNaiveDomains) != len(domainSlice) {
				t.Errorf("len(domains) = %d, expected %d", len(domainSlice), len(tt.expectNaiveDomains))
			} else {
				for _, expectedDomain := range tt.expectNaiveDomains {
					if !domainMap.ExactMatch(expectedDomain) {
						t.Errorf("expected domain %q not found in domains", expectedDomain)
					}
				}
			}

			// Check URLs
			if len(tt.expectURLs) != len(globalMatcher.urls) {
				t.Errorf("len(globalMatcher.urls) = %d, expected %d", len(globalMatcher.urls), len(tt.expectURLs))
			} else {
				for i, url := range tt.expectURLs {
					if globalMatcher.urls[i].String() != url {
						t.Errorf("globalMatcher.urls[%d] = %q, expected %q", i, globalMatcher.urls[i].String(), url)
					}
				}
			}

			// Check regexes
			if len(tt.expectRegexes) != len(globalMatcher.regexes) {
				t.Errorf("len(globalMatcher.regexes) = %d, expected %d", len(globalMatcher.regexes), len(tt.expectRegexes))
			} else {
				for i, re := range tt.expectRegexes {
					if globalMatcher.regexes[i].String() != re {
						t.Errorf("globalMatcher.regexes[%d] = %q, expected %q", i, globalMatcher.regexes[i].String(), re)
					}
				}
			}
		})
	}
}

// Test Match function
func TestMatch(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		elements []string
		expected bool
	}{
		{
			name:     "Exact match for naive domain",
			rawURL:   "https://example.com",
			elements: []string{"example.com"},
			expected: true,
		},
		{
			name:     "Subdomain match for naive domain",
			rawURL:   "https://sub.example.com",
			elements: []string{"example.com"},
			expected: true,
		},
		{
			name:     "Exact match for full URL",
			rawURL:   "https://example.org/path?query=1",
			elements: []string{"https://example.org/path?query=1"},
			expected: true,
		},
		{
			name:     "No match for full URL",
			rawURL:   "https://example.org/path?query=completely-different",
			elements: []string{"https://example.org/path?query=1"},
			expected: false,
		},
		{
			name:     "Greedy match for naive domain",
			rawURL:   "https://example.org/path?query=1",
			elements: []string{"example.org"},
			expected: true,
		},
		{
			name:     "Greedy match for full URL",
			rawURL:   "https://example.org/path?query=1",
			elements: []string{"https://example.org"},
			expected: true,
		},
		{
			name:     "Regex match",
			rawURL:   "https://example.net/",
			elements: []string{`^https?://(www\.)?example\.net/.*`},
			expected: true,
		},
		{
			name:     "Regex match with different scheme",
			rawURL:   "http://www.example.net/resource",
			elements: []string{`^https?://(www\.)?example\.net/.*`},
			expected: true,
		},
		{
			name:     "No match for different domain",
			rawURL:   "https://different.com",
			elements: []string{"example.com"},
			expected: false,
		},
		{
			name:     "No match for different full URL",
			rawURL:   "https://example.com/path",
			elements: []string{"https://another-example.com"},
			expected: false,
		},
		{
			name:     "No match for different regex",
			rawURL:   "https://example.net/",
			elements: []string{`^https?://(www\.)?example\.com/.*`},
			expected: false,
		},
		{
			name:     "No match for different precise regex",
			rawURL:   "https://example.net/?query=1",
			elements: []string{`^https?://(www\.)?example\.net/only-one-path$`},
			expected: false,
		},
		{
			name:     "Invalid URL with valid naive domain",
			rawURL:   "%am-i-really-an-URL?",
			elements: []string{"example.com"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Reset()

			err := AddElements(tt.elements, nil)
			if err != nil {
				t.Fatalf("Failed to add elements: %v", err)
			}

			result := Match(tt.rawURL)
			if result != tt.expected {
				t.Errorf("Match(%q) = %v, expected %v", tt.rawURL, result, tt.expected)
			}
		})
	}
}
