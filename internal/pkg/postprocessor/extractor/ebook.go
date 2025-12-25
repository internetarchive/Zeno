package extractor

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	neturl "net/url"

	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/pkg/models"
)

type EbookOutlinkExtractor struct{}

func (EbookOutlinkExtractor) Support(m Mode) bool {
	return m == ModeGeneral
}

func (EbookOutlinkExtractor) Match(URL *models.URL) bool {
	m := URL.GetMIMEType()
	if m != nil && m.Is("application/epub+zip") {
		return true
	}
	if p := URL.GetParsed(); p != nil {
		return strings.HasSuffix(strings.ToLower(p.Path), ".epub")
	}
	return strings.HasSuffix(strings.ToLower(URL.String()), ".epub")
}

func (EbookOutlinkExtractor) Extract(URL *models.URL) (outlinks []*models.URL, err error) {
	defer URL.RewindBody()

	body := URL.GetBody()
	if body == nil {
		return nil, fmt.Errorf("ebook extractor: nil body")
	}

	var buf bytes.Buffer
	if _, e := io.Copy(&buf, body); e != nil {
		return nil, fmt.Errorf("ebook extractor: reading body: %w", e)
	}

	zr, e := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if e != nil {
		return nil, fmt.Errorf("ebook extractor: opening zip: %w", e)
	}

	opfPath, findErr := findOPFPath(zr)
	if findErr != nil {
		opfPath, findErr = findAnyOPF(zr)
		if findErr != nil {
			return nil, fmt.Errorf("ebook extractor: locate opf: %w", findErr)
		}
	}

	opfFile, e := openZipFile(zr, opfPath)
	if e != nil {
		return nil, fmt.Errorf("ebook extractor: open opf %s: %w", opfPath, e)
	}
	opfBytes, e := io.ReadAll(opfFile)
	opfFile.Close()
	if e != nil {
		return nil, fmt.Errorf("ebook extractor: read opf %s: %w", opfPath, e)
	}

	pkg, e := parseOPF(opfBytes)
	if e != nil {
		return nil, fmt.Errorf("ebook extractor: parse opf %s: %w", opfPath, e)
	}

	idToHref := map[string]string{}
	for _, it := range pkg.Manifest.Items {
		idToHref[it.ID] = it.Href
	}

	opfBase := filepath.ToSlash(path.Dir(opfPath))
	if opfBase == "." {
		opfBase = ""
	}

	var errs []string

	for _, ir := range pkg.Spine.Itemrefs {
		href, ok := idToHref[ir.IDRef]
		if !ok || href == "" {
			errs = append(errs, fmt.Sprintf("missing manifest for idref %s", ir.IDRef))
			continue
		}

		itemPath := filepath.ToSlash(path.Join(opfBase, href))
		r, e := openZipFile(zr, itemPath)
		if e != nil {
			errs = append(errs, fmt.Sprintf("open spine item %s: %v", itemPath, e))
			continue
		}

		doc, e := goquery.NewDocumentFromReader(r)
		r.Close()
		if e != nil {
			errs = append(errs, fmt.Sprintf("parse html %s: %v", itemPath, e))
			continue
		}

		baseParsed := URL.GetParsed()
		if baseParsed == nil && URL.GetRequest() != nil && URL.GetRequest().URL != nil {
			baseParsed = URL.GetRequest().URL
		}
		if baseParsed == nil {
			if p, perr := neturl.Parse(URL.Raw); perr == nil {
				baseParsed = p
			}
		}
		if baseParsed == nil {
			errs = append(errs, fmt.Sprintf("cannot determine base URL for resolving links in %s", itemPath))
			continue
		}

		itemDir := path.Dir(itemPath)
		baseStr := strings.TrimRight(baseParsed.String(), "/") + "/"
		if itemDir != "" && itemDir != "." {
			baseStr = strings.TrimRight(baseStr, "/") + "/" + strings.TrimLeft(itemDir, "/") + "/"
		}
		baseURL, _ := neturl.Parse(baseStr)

		doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
			hrefVal, exists := s.Attr("href")
			if !exists || strings.TrimSpace(hrefVal) == "" {
				return
			}
			if strings.HasPrefix(hrefVal, "javascript:") || strings.HasPrefix(hrefVal, "data:") {
				return
			}

			parsed, perr := neturl.Parse(hrefVal)
			if perr != nil {
				errs = append(errs, fmt.Sprintf("invalid href %q in %s: %v", hrefVal, itemPath, perr))
				return
			}
			resolved := baseURL.ResolveReference(parsed)
			outlinks = append(outlinks, &models.URL{Raw: resolved.String()})
		})
	}

	if len(errs) > 0 {
		return outlinks, fmt.Errorf("ebook extractor: partial errors: %s", strings.Join(errs, "; "))
	}
	return outlinks, nil
}

type opfPackage struct {
	XMLName  xml.Name    `xml:"package"`
	Manifest opfManifest `xml:"manifest"`
	Spine    opfSpine    `xml:"spine"`
}

type opfManifest struct {
	Items []opfManifestItem `xml:"item"`
}

type opfManifestItem struct {
	ID        string `xml:"id,attr"`
	Href      string `xml:"href,attr"`
	MediaType string `xml:"media-type,attr"`
}

type opfSpine struct {
	Itemrefs []opfSpineItemref `xml:"itemref"`
}

type opfSpineItemref struct {
	IDRef string `xml:"idref,attr"`
}

func findOPFPath(zr *zip.Reader) (string, error) {
	const containerPath = "META-INF/container.xml"
	f, err := openZipFile(zr, containerPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", containerPath, err)
	}
	b, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		return "", fmt.Errorf("read %s: %w", containerPath, err)
	}

	type rootfile struct {
		FullPath string `xml:"full-path,attr"`
	}
	type container struct {
		Rootfiles struct {
			Files []rootfile `xml:"rootfile"`
		} `xml:"rootfiles"`
	}

	var c container
	if err := xml.Unmarshal(b, &c); err != nil {
		clean := stripXMLNS(b)
		if err2 := xml.Unmarshal(clean, &c); err2 != nil {
			return "", fmt.Errorf("parse container.xml: %v (fallback: %v)", err, err2)
		}
	}
	if len(c.Rootfiles.Files) == 0 {
		return "", fmt.Errorf("no rootfile in container.xml")
	}
	return filepath.ToSlash(c.Rootfiles.Files[0].FullPath), nil
}

func findAnyOPF(zr *zip.Reader) (string, error) {
	for _, f := range zr.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".opf") {
			return filepath.ToSlash(f.Name), nil
		}
	}
	return "", fmt.Errorf("no .opf file found in epub")
}

func parseOPF(opfBytes []byte) (*opfPackage, error) {
	var pkg opfPackage
	if err := xml.Unmarshal(opfBytes, &pkg); err != nil {
		clean := stripXMLNS(opfBytes)
		if err2 := xml.Unmarshal(clean, &pkg); err2 != nil {
			return nil, fmt.Errorf("unmarshal opf: %v (fallback: %v)", err, err2)
		}
	}
	return &pkg, nil
}

func openZipFile(zr *zip.Reader, name string) (io.ReadCloser, error) {
	normalized := filepath.ToSlash(name)
	for _, f := range zr.File {
		if filepath.ToSlash(f.Name) == normalized {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			return rc, nil
		}
	}
	return nil, fmt.Errorf("file %s not found in epub", name)
}

func stripXMLNS(b []byte) []byte {
	s := string(b)
	s = strings.ReplaceAll(s, `xmlns="http://www.idpf.org/2007/opf"`, "")
	s = strings.ReplaceAll(s, `xmlns="http://purl.org/dc/elements/1.1/"`, "")
	return []byte(removeXMLNSPrefixes(s))
}

func removeXMLNSPrefixes(s string) string {
	for {
		start := strings.Index(s, "xmlns:")
		if start == -1 {
			break
		}
		eqIndex := strings.Index(s[start:], "=")
		if eqIndex == -1 {
			break
		}
		quoteStart := start + eqIndex + 1
		if quoteStart >= len(s) {
			break
		}
		quoteChar := s[quoteStart]
		if quoteChar != '"' && quoteChar != '\'' {
			s = s[:start] + s[start+6:]
			continue
		}
		quoteEnd := strings.IndexByte(s[quoteStart+1:], quoteChar)
		if quoteEnd == -1 {
			break
		}
		quoteEnd = quoteStart + 1 + quoteEnd
		s = s[:start] + s[quoteEnd+1:]
	}
	return s
}
