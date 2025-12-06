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

	"net/url"

	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/pkg/models"
)

type EbookOutlinkExtractor struct{}

func (EbookOutlinkExtractor) Support(m Mode) bool {
	return m == ModeGeneral
}

func (EbookOutlinkExtractor) Match(URL *models.URL) bool {
	m := URL.GetMIMEType()
	return m.Is("application/epub+zip") || strings.HasSuffix(strings.ToLower(URL.GetParsed().Path), ".epub")
}

func (EbookOutlinkExtractor) Extract(URL *models.URL) (outlinks []*models.URL, err error) {
	// Ensure the body can be rewound for downstream processors.
	defer URL.RewindBody()

	// Read whole body into buffer because archive/zip requires random access.
	body := URL.GetBody()
	if body == nil {
		return nil, fmt.Errorf("ebook extractor: nil body")
	}

	// Read all bytes from body into buf
	var buf bytes.Buffer
	if _, e := io.Copy(&buf, body); e != nil {
		return nil, fmt.Errorf("Ebook extractor: reading body: %w", e)
	}

	// Create a zip reader from the bytes buffer
	zr, e := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if e != nil {
		return nil, fmt.Errorf("EBook extractor: opening zip: %w", e)
	}

	//Find OPF path via META-INF/container.xml
	opfPath, findErr := findOPFPath(zr)
	if findErr != nil {
		// If we couldn't find container.xml or OPF path, attempt fallback: search for *.opf
		opfPath, findErr = findAnyOPF(zr)
		if findErr != nil {
			// Fail early if no OPF found
			return nil, fmt.Errorf("ebook extractor: locate opf: %w", findErr)
		}
	}

	opfFile, e := openZipFile(zr, opfPath)
	if e != nil {
		return nil, fmt.Errorf("ebook extractor: open opf %s: %w", opfPath, e)
	}
	opfBytes, e := io.ReadAll(opfFile)
	if e != nil {
		return nil, fmt.Errorf("ebook extractor: read opf %s: %w", opfPath, e)
	}

	// Parse OPF to get manifest + spine
	pkg, e := parseOPF(opfBytes)
	if e != nil {
		return nil, fmt.Errorf("eook extractor: parse opf %s: %w", opfPath, e)
	}

	// Build id -> href map from manifest
	idToHref := map[string]string{}
	for _, it := range pkg.Manifest.Items {
		idToHref[it.ID] = it.Href
	}

	// Base dir for relative hrefs inside EPUB
	opfBase := filepath.ToSlash(path.Dir(opfPath))
	if opfBase == "." {
		opfBase = ""
	}

	// Collect errors but continue processing remaining spine items
	var errs []string

	// For each itemref in spine, resolve href and process the file
	for _, ir := range pkg.Spine.Itemrefs {
		href, ok := idToHref[ir.IDRef]
		if !ok || href == "" {
			// missing manifest entry — record and continue
			errs = append(errs, fmt.Sprintf("missing manifest for idref %s", ir.IDRef))
			continue
		}

		// Resolve href relative to opfBase
		itemPath := filepath.ToSlash(path.Join(opfBase, href))

		// Open item in zip
		r, e := openZipFile(zr, itemPath)
		if e != nil {
			// Record and continue
			errs = append(errs, fmt.Sprintf("open spine item %s: %v", itemPath, e))
			continue
		}

		// Parse HTML/XHTML content with goquery
		doc, e := goquery.NewDocumentFromReader(r)
		if e != nil {
			errs = append(errs, fmt.Sprintf("parse html %s: %v", itemPath, e))
			continue
		}

		// Determine base URL for resolving relative links
		// and the item's directory path to resolve relative hrefs
		baseParsed := URL.GetParsed()
		// Create a pseudo base for this item: baseURL + path.Dir(opfBase/href)
		itemDir := path.Dir(itemPath)
		// Create a string base (not used to fetch, only for resolution)
		baseStr := baseParsed.String()
		if itemDir != "" && itemDir != "." {
			// Ensure we join with a trailing slash so ResolveReference works for relative paths
			baseStr = strings.TrimRight(baseStr, "/") + "/" + strings.TrimLeft(itemDir, "/") + "/"
		} else {
			baseStr = strings.TrimRight(baseStr, "/") + "/"
		}

		baseURL, _ := url.Parse(baseStr)

		// Extract <a href> elements
		doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
			hrefVal, exists := s.Attr("href")
			if !exists || strings.TrimSpace(hrefVal) == "" {
				return
			}

			// Skip fragments or javascript/data schemes quickly
			if strings.HasPrefix(hrefVal, "javascript:") || strings.HasPrefix(hrefVal, "data:") {
				return
			}

			// Resolve relative links against baseURL
			parsed, perr := url.Parse(hrefVal)
			if perr != nil {
				// if cannot parse, record and skip
				errs = append(errs, fmt.Sprintf("invalid href %q in %s: %v", hrefVal, itemPath, perr))
				return
			}
			resolved := baseURL.ResolveReference(parsed)

			// Append as models.URL with Raw set to resolved.String()
			outlinks = append(outlinks, &models.URL{Raw: resolved.String()})
		})
	}

	// Consolidate errors (if any) into a single error return (preserve outlinks)
	if len(errs) > 0 {
		return outlinks, fmt.Errorf("ebook extractor: partial errors: %s", strings.Join(errs, "; "))
	}
	return outlinks, nil
}

// minimal OPF structs for parsing manifest+spine
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

// findOPFPath reads META-INF/container.xml to locate the package (OPF) path
func findOPFPath(zr *zip.Reader) (string, error) {
	const containerPath = "META-INF/container.xml"
	f, err := openZipFile(zr, containerPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", containerPath, err)
	}
	b, err := io.ReadAll(f)
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
		return "", fmt.Errorf("parse container.xml: %w", err)
	}
	if len(c.Rootfiles.Files) == 0 {
		return "", fmt.Errorf("no rootfile in container.xml")
	}
	// use the first rootfile
	return filepath.ToSlash(c.Rootfiles.Files[0].FullPath), nil
}

// fallback: find any .opf file in the zip
func findAnyOPF(zr *zip.Reader) (string, error) {
	for _, f := range zr.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".opf") {
			return filepath.ToSlash(f.Name), nil
		}
	}
	return "", fmt.Errorf("no .opf file found in epub")
}

// parseOPF parses OPF package bytes into opfPackage struct
func parseOPF(opfBytes []byte) (*opfPackage, error) {
	var pkg opfPackage
	// OPF files often include default namespace; use decoder that strips it
	decoder := xml.NewDecoder(bytes.NewReader(opfBytes))
	// Remove namespace prefixes by setting the element name's Local
	for {
		t, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("xml token error: %w", err)
		}
		_ = t
	}
	// Simple unmarshal
	if err := xml.Unmarshal(opfBytes, &pkg); err != nil {
		// try a fallback by removing xmlns attributes to avoid namespace issues
		clean := stripXMLNS(opfBytes)
		if err2 := xml.Unmarshal(clean, &pkg); err2 != nil {
			return nil, fmt.Errorf("unmarshal opf: %v (fallback: %v)", err, err2)
		}
	}
	return &pkg, nil
}

// openZipFile returns a ReadCloser for a given path inside the zip (exact match)
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

// stripXMLNS removes xmlns declarations crudely to help xml.Unmarshal without namespace handling.
func stripXMLNS(b []byte) []byte {
	s := string(b)
	// remove typical xmlns="..." occurrences
	s = strings.ReplaceAll(s, `xmlns="http://www.idpf.org/2007/opf"`, "")
	s = strings.ReplaceAll(s, `xmlns="http://purl.org/dc/elements/1.1/"`, "")
	// remove any xmlns:prefix="..." patterns (basic)

	s = removeXMLNSPrefixes(s)
	return []byte(s)
}

func removeXMLNSPrefixes(s string) string {
	// crude loop to remove patterns
	for {
		start := strings.Index(s, "xmlns:")
		if start == -1 {
			break
		}
		end := strings.Index(s[start:], "\"")
		if end == -1 {
			break
		}
		// find closing quote
		rest := s[start+end+1:]
		end2 := strings.Index(rest, "\"")
		if end2 == -1 {
			break
		}

		s = s[:start] + s[start+1+strings.Index(s[start+1:], " "):] // best-effort trim

		break
	}
	return s
}
