package cmd

import (
	"slices"
	"strings"
	"testing"
)

// A line that is not a valid URL is skipped instead of stopping the whole crawl when the seeds are queued.
func TestReadURLListSkipsInvalidURLs(t *testing.T) {
	list := "https://example.com/\n" +
		"\n" +
		"# comment\n" +
		"  https://example.com/page1  \r\n" +
		"http://water.epa.gov/ty\x18\n" +
		"example.com/no-scheme\n" +
		"https://example.com/page2\n"

	urls, err := readURLList(strings.NewReader(list), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"https://example.com/", "https://example.com/page1", "https://example.com/page2"}
	if !slices.Equal(urls, expected) {
		t.Errorf("expected %q, got %q", expected, urls)
	}
}
