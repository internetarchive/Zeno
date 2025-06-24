package extractor

import (
	"slices"
	"testing"
)

func TestHasFileExtension(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "Simple JPG extension",
			input: "http://example.com/image.jpg",
			want:  true,
		},
		{
			name:  "Query param after extension",
			input: "https://example.org/dog.png?foo=bar",
			want:  true,
		},
		{
			name:  "Fragment after extension",
			input: "https://test.com/cat.gif#section1",
			want:  true,
		},
		{
			name:  "No extension at all",
			input: "http://example.com/foo",
			want:  false,
		},
		{
			name:  "Trailing slash after potential extension",
			input: "http://example.com/foo.txt/",
			want:  false, // The extension is not truly at the end
		},
		{
			name:  "Extension deeper in path",
			input: "http://example.com/data.txt/archive",
			want:  false, // The .txt is not the last segment
		},
		{
			name:  "Multiple dots, multiple segments",
			input: "http://example.net/backups/data.tar.gz?version=2",
			want:  true,
		},
		{
			name:  "Hidden file style, no extension (e.g. .htaccess)",
			input: "https://example.com/.htaccess",
			want:  true,
		},
		{
			name:  "Dot at the end only (no extension)",
			input: "http://example.org/name.",
			want:  false, // There's no extension after the final dot
		},
		{
			name:  "Just a plain filename with extension, no slashes",
			input: "file.zip",
			want:  true,
		},
		{
			name:  "Filename with multiple dots in the last segment",
			input: "https://example.io/some.dir/my.file.name.txt",
			want:  true,
		},
		{
			name:  "Parameters but no dot in final segment",
			input: "https://example.com/paramCheck?this=that",
			want:  false,
		},
		{
			name:  "Multiple slashes near the end",
			input: "http://example.com/dir/subdir/.hidden/",
			want:  false,
		},
		{
			name:  "Dot in subdirectory name only",
			input: "http://example.com/dir.withdot/filename",
			want:  false,
		},
		{
			name:  "Extension is the last item plus fragment",
			input: "http://example.com/test.db#backup",
			want:  true,
		},
		{
			name:  "No slash, no dot, random string",
			input: "thisIsJustAString",
			want:  false,
		},
		{
			name:  "Multiple dots in final segment with a trailing query",
			input: "http://example.com/foo.bar.baz.qux?stuff=1",
			want:  true,
		},
		{
			name:  "Extension disguised with a slash in the query",
			input: "http://example.com/data.zip?path=/etc/passwd",
			want:  true,
		},
		{
			name:  "Protocol and domain only, no path",
			input: "https://example.com",
			want:  false,
		},
		{
			name:  "Protocol and domain with trailing slash",
			input: "https://example.com/",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasFileExtension(tt.input)
			if got != tt.want {
				t.Errorf("hasFileExtension(%q) = %v; want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestLinkRegex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Valid URL",
			input:    "Check this link: https://example.com",
			expected: []string{"https://example.com"},
		},
		{
			name:     "Multiple URLs",
			input:    "Links: http://example.com, https://test.org/path",
			expected: []string{"http://example.com", "https://test.org/path"},
		},
		{
			name:     "No URLs",
			input:    "Just some text without links.",
			expected: []string{},
		},
		{
			name:     "Bare domain without protocol",
			input:    "This is not a valid link: example.com",
			expected: []string{},
		},
		{
			name:     "URL",
			input:    "This is a link: https://example.com/path/to/resource?query=param#fragment and some text.",
			expected: []string{"https://example.com/path/to/resource?query=param#fragment"},
		},
		{
			name:  "Markdown-style link",
			input: "Check this [link](https://example.com/1.html) for more info. details: <https://example.com/2.html>",
			expected: []string{
				"https://example.com/1.html)", // <-- This is a trade-off, if I let the regex to ignore the closing parenthesis,
				// it will not match the below "Wikipedia URL" case correctly.
				// But [LinkRegexStrict] would match both URLs correctly, IDK how it works unfortunately :(
				"https://example.com/2.html",
			},
		},
		{
			name:     "Wikipedia URL",
			input:    "page: https://en.wikipedia.org/wiki/Pipeline_(Unix) and some text.",
			expected: []string{"https://en.wikipedia.org/wiki/Pipeline_(Unix)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LinkRegex.FindAllString(tt.input, -1)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d URLs, got %d", len(tt.expected), len(result))
				return
			}
			slices.Sort(result)
			slices.Sort(tt.expected)
			for i, url := range result {
				if url != tt.expected[i] {
					t.Errorf("Expected URL %s, got %s", tt.expected[i], url)
				}
			}
		})
	}
}

func TestQuotedLinkRegex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Quoted URL",
			input:    `Check this link: 'https://example.com'`,
			expected: []string{"https://example.com"},
		},
		{
			name:     "Multiple quoted URLs",
			input:    `Links: "http://example.com", "https://test.org/path"`,
			expected: []string{"http://example.com", "https://test.org/path"},
		},
		{
			name:  "No quoted URLs",
			input: `Just some text without links.`,
		},
		{
			name:  "Unquoted URL",
			input: `This is not a valid link: https://example.com`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := QuotedLinkRegexFindAll(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d quoted URLs, got %d", len(tt.expected), len(result))
				return
			}

			slices.Sort(result)
			slices.Sort(tt.expected)
			for i, url := range result {
				if url != tt.expected[i] {
					t.Errorf("Expected quoted URL %s, got %s", tt.expected[i], url)
				}
			}
		})
	}
}
