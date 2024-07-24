package crawl

import (
	"slices"
	"testing"
)

func TestParseOneLink(t *testing.T) {
	var links []Link
	links = append(links, Link{URL: "https://one.example.com", Rel: "preconnect"})

	var link = `<https://one.example.com>; rel="preconnect"`

	got := Parse(link)
	want := links

	if !slices.Equal(got, want) {
		t.Errorf("got %q, wanted %q", got, want)
	}
}

func TestParseMultipleLinks(t *testing.T) {

	var links []Link
	links = append(links,
		Link{URL: "https://test.com", Rel: "preconnect"},
		Link{URL: "https://app.test.com", Rel: "preconnect"},
		Link{URL: "https://example.com", Rel: "preconnect"},
	)

	var link = `<https://test.com>; rel="preconnect", <https://app.test.com>; rel="preconnect"; foo="bar", <https://example.com>; rel="preconnect"`

	got := Parse(link)
	want := links

	if !slices.Equal(got, want) {
		t.Errorf("got %q, wanted %q", got, want)
	}
}
