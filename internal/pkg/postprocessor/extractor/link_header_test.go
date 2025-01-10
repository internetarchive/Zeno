package extractor

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/internetarchive/Zeno/pkg/models"
)

func TestExtractURLsFromHeader(t *testing.T) {
	tests := []struct {
		name     string
		link     string
		expected []*models.URL
	}{
		{
			name: "Valid Link header with multiple URLs",
			link: `<https://one.example.com>; rel="preconnect", <https://two.example.com>; rel="preload"`,
			expected: []*models.URL{
				{Raw: "https://one.example.com"},
				{Raw: "https://two.example.com"},
			},
		},
		{
			name:     "Valid Link header with no URLs",
			link:     ``,
			expected: nil,
		},
		{
			name: "Malformed Link header",
			link: `https://one.example.com>;; rel=preconnect";`,
			expected: []*models.URL{
				{Raw: "https://one.example.com"},
			},
		},
		{
			name: "Link header with nested elements containing URLs",
			link: `<https://example.com/nested>; rel="preconnect"`,
			expected: []*models.URL{
				{Raw: "https://example.com/nested"},
			},
		},
		{
			name: "Link header with attributes containing URLs",
			link: `<https://example.com/attr>; rel="preconnect"`,
			expected: []*models.URL{
				{Raw: "https://example.com/attr"},
			},
		},
		{
			name: "Link header with mixed content",
			link: `<https://example.com/mixed>; rel="preconnect"`,
			expected: []*models.URL{
				{Raw: "https://example.com/mixed"},
			},
		},
		{
			name: "Large Link header content",
			link: func() string {
				var link string
				for i := 0; i < 1000; i++ {
					link += fmt.Sprintf("<https://example.com/page%d>; rel=\"preconnect\", ", i)
				}
				return link[:len(link)-2]
			}(),
			expected: func() []*models.URL {
				var urls []*models.URL
				for i := 0; i < 1000; i++ {
					urls = append(urls, &models.URL{Raw: fmt.Sprintf("https://example.com/page%d", i)})
				}
				return urls
			}(),
		},
		{
			name: "Link header with special characters in URLs",
			link: `<https://example.com/page?param=1&other=2>; rel="preconnect"`,
			expected: []*models.URL{
				{Raw: "https://example.com/page?param=1&other=2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractURLsFromHeader(tt.link)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Fatalf("ExtractURLsFromHeader() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseAttr(t *testing.T) {
	tests := []struct {
		attr      string
		wantKey   string
		wantValue string
	}{
		{
			attr:      `rel="preconnect"`,
			wantKey:   "rel",
			wantValue: "preconnect",
		},
		{
			attr:      `="preconnect"`,
			wantKey:   "",
			wantValue: "preconnect",
		},
		{
			attr:      `foo="bar"`,
			wantKey:   "foo",
			wantValue: "bar",
		},
		{
			attr:      `key="value"`,
			wantKey:   "key",
			wantValue: "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.attr, func(t *testing.T) {
			gotKey, gotValue := parseAttr(tt.attr)
			if gotKey != tt.wantKey {
				t.Fatalf("parseAttr() gotKey = %v, want %v", gotKey, tt.wantKey)
			}
			if gotValue != tt.wantValue {
				t.Fatalf("parseAttr() gotValue = %v, want %v", gotValue, tt.wantValue)
			}
		})
	}
}
