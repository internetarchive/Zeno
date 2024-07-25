package utils

import (
	"net/url"
	"testing"
)

func TestURLToString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
		expected  string
	}{
		{
			name:      "Valid URL with query",
			input:     "http://bing.com/search?q=dotnet",
			expectErr: false,
			expected:  "http://bing.com/search?q=dotnet",
		},
		{
			name:      "Valid URL without query",
			input:     "http://example.com",
			expectErr: false,
			expected:  "http://example.com",
		},
		{
			name:      "URL with port",
			input:     "http://localhost:8080",
			expectErr: false,
			expected:  "http://localhost:8080",
		},
		{
			name:      "URL with path",
			input:     "http://example.com/path/to/resource",
			expectErr: false,
			expected:  "http://example.com/path/to/resource",
		},
		{
			name:      "Invalid URL without scheme",
			input:     "://missing-scheme.com",
			expectErr: true,
			expected:  "",
		},
		{
			name:      "URL with fragment",
			input:     "http://example.com/path#section",
			expectErr: false,
			expected:  "http://example.com/path#section",
		},
		{
			name:      "URL with IDN domain",
			input:     "http://xn--fsq.com",
			expectErr: false,
			expected:  "http://xn--fsq.com",
		},
		{
			name:      "URL with unicode domain",
			input:     "http://bücher.de",
			expectErr: false,
			expected:  "http://xn--bcher-kva.de",
		},
		{
			name:      "URL with another unicode domain",
			input:     "http://faß.de",
			expectErr: false,
			expected:  "http://xn--fa-hia.de",
		},
		{
			name:      "URL with IPv6 address",
			input:     "http://[2001:db8::1]:8080",
			expectErr: false,
			expected:  "http://[2001:db8::1]:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.input)
			if (err != nil) != tt.expectErr {
				t.Fatalf("Error parsing input string: %s, err: %v", tt.input, err)
			}
			if err == nil && tt.expectErr {
				t.Fatalf("Expected an error for input string: %s, but got none", tt.input)
			}
			if err == nil {
				result := URLToString(u)
				if result != tt.expected {
					t.Errorf("Result was incorrect, got: %s, want: %s", result, tt.expected)
				}
			}
		})
	}
}
