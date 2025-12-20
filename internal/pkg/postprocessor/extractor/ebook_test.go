package extractor

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/url"
	"testing"

	"github.com/internetarchive/Zeno/pkg/models"
)

type mockReadSeekCloser struct {
	*bytes.Reader
	name string
}

func (m mockReadSeekCloser) Close() error     { return nil }
func (m mockReadSeekCloser) FileName() string { return m.name }

func makeMinimalEPUB(link string) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	add := func(name string, data []byte) error {
		f, err := w.Create(name)
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return err
	}

	container := `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf"/>
  </rootfiles>
</container>`

	opf := `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <manifest>
    <item id="item1" href="chapter1.html" media-type="application/xhtml+xml"/>
  </manifest>
  <spine toc="ncx">
    <itemref idref="item1"/>
  </spine>
</package>`

	html := `<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <body>
    <a href="` + link + `">external</a>
  </body>
</html>`

	if err := add("META-INF/container.xml", []byte(container)); err != nil {
		w.Close()
		return nil, err
	}
	if err := add("OEBPS/content.opf", []byte(opf)); err != nil {
		w.Close()
		return nil, err
	}
	if err := add("OEBPS/chapter1.html", []byte(html)); err != nil {
		w.Close()
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestEbookOutlinkExtractor_Match(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"match epub lowercase", "test.epub", true},
		{"match epub uppercase", "test.EPUB", true},
		{"non-ebook extension", "test.txt", false},
		{"similar extension", "ebook.epubbak", false},
	}

	extractor := EbookOutlinkExtractor{}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			URLObj := &models.URL{}
			URLObj.SetRequest(&http.Request{URL: &url.URL{Scheme: "http", Host: "example.com"}})
			tmpFile := mockReadSeekCloser{
				Reader: bytes.NewReader([]byte("dummy")),
				name:   tc.filename,
			}
			URLObj.SetBody(tmpFile)
			got := extractor.Match(URLObj)
			if got != tc.want {
				t.Fatalf("Match() = %v, want %v for filename %q", got, tc.want, tc.filename)
			}
		})
	}
}

func TestEbookOutlinkExtractor_Extract_minimalEPUB(t *testing.T) {
	extractor := EbookOutlinkExtractor{}

	link := "http://example.org/target"
	zipBytes, err := makeMinimalEPUB(link)
	if err != nil {
		t.Fatalf("makeMinimalEPUB: %v", err)
	}

	URLObj := &models.URL{}
	URLObj.SetRequest(&http.Request{URL: &url.URL{Scheme: "http", Host: "example.com", Path: "/test.epub"}})
	tmpFile := mockReadSeekCloser{
		Reader: bytes.NewReader(zipBytes),
		name:   "test.epub",
	}
	URLObj.SetBody(tmpFile)

	outlinks, err := extractor.Extract(URLObj)
	if err != nil && len(outlinks) == 0 {
		t.Fatalf("Extract returned error and no outlinks: %v", err)
	}
	if len(outlinks) == 0 {
		t.Fatalf("no outlinks extracted")
	}
	found := false
	for _, u := range outlinks {
		if u != nil && u.Raw == link {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected extracted link %q not found in outlinks: %+v", link, outlinks)
	}
}
