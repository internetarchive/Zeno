package models

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/CorentinB/warc/pkg/spooledtempfile"
	"github.com/PuerkitoBio/goquery"
	"github.com/gabriel-vasile/mimetype"
	"golang.org/x/net/html/charset"
	"golang.org/x/net/idna"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
)

type URL struct {
	Raw       string
	parsed    *url.URL
	request   *http.Request
	response  *http.Response
	body      spooledtempfile.ReadSeekCloser
	document  *goquery.Document
	mimetype  *mimetype.MIME
	Hops      int // This determines the number of hops this item is the result of, a hop is a "jump" from 1 page to another page
	Redirects int

	stringCache string
	once        sync.Once
	documentMu  sync.Mutex // Protect document parsing
}

func (u *URL) Parse() (err error) {
	u.parsed, err = url.ParseRequestURI(u.Raw)
	return err
}

func (u *URL) GetBody() spooledtempfile.ReadSeekCloser {
	return u.body
}

func (u *URL) SetBody(body spooledtempfile.ReadSeekCloser) {
	u.body = body
}

func (u *URL) NormalizeURL(rawURL string) (string, error) {
	if rawURL == "" {
		return rawURL, nil
	}

	// Handle special cases like data: and javascript: URLs
	if strings.HasPrefix(rawURL, "data:") || strings.HasPrefix(rawURL, "javascript:") {
		return rawURL, nil
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		// Try to handle partially encoded URLs
		decodedURL := rawURL

		// Attempt to safely decode any percent-encoded sequences
		// if possible in case of multiple encodings
		// (this should be uncommon and is defensive)
		for attempt := 0; attempt < 3; attempt++ {
			prev := decodedURL
			tmpDecoded, tmpErr := url.QueryUnescape(decodedURL)
			if tmpErr != nil {
				break
			}
			decodedURL = tmpDecoded
			if decodedURL == prev {
				break
			}
		}

		// Try parsing again with our best attempt at decoding.
		// If all else fails, return the original URL.
		parsedURL, err = url.Parse(decodedURL)
		if err != nil {
			return rawURL, err
		}
	}

	var normalizedURL strings.Builder
	if parsedURL.Scheme != "" {
		normalizedURL.WriteString(parsedURL.Scheme)
		normalizedURL.WriteString("://")
	}

	if parsedURL.Host != "" {
		// For IDN domains, make sure we're using the Unicode representation
		host := parsedURL.Host
		if strings.HasPrefix(host, "xn--") {
			ascii, err := idna.ToUnicode(host)
			if err == nil {
				host = ascii
			}
		}
		normalizedURL.WriteString(host)
	}

	// Add path (properly decoded to preserve Unicode)
	if parsedURL.Path != "" {
		if parsedURL.Host != "" && !strings.HasPrefix(parsedURL.Path, "/") {
			normalizedURL.WriteString("/")
		}
		decodedPath, err := url.PathUnescape(parsedURL.Path)
		if err == nil {
			normalizedURL.WriteString(decodedPath)
		} else {
			normalizedURL.WriteString(parsedURL.Path)
		}
	}

	// Add query and then fragment
	if parsedURL.RawQuery != "" {
		normalizedURL.WriteString("?")
		normalizedURL.WriteString(parsedURL.RawQuery)
	}

	if parsedURL.Fragment != "" {
		normalizedURL.WriteString("#")
		normalizedURL.WriteString(parsedURL.Fragment)
	}

	return normalizedURL.String(), nil
}

func createDecodedReader(body io.Reader, contentType string) (io.Reader, error) {
	if seeker, ok := body.(io.Seeker); ok {
		_, err := seeker.Seek(0, io.SeekStart)
		if err != nil {
			return nil, fmt.Errorf("failed to rewind body: %w", err)
		}
	}

	bufferedReader := bufio.NewReader(body)

	// Detect charset from content by peeking
	var detectedCharset string
	if data, err := bufferedReader.Peek(1024); err == nil {
		if _, name, ok := charset.DetermineEncoding(data, contentType); ok {
			detectedCharset = name
			slog.Debug("Detected charset", "charset", detectedCharset, "content-type", contentType)
		}
	}

	// If no charset detected, try from content-type header
	if detectedCharset == "" {
		detectedCharset = getCharsetFromContentType(contentType)
	}

	// If still no charset, default to UTF-8
	if detectedCharset == "" {
		detectedCharset = "utf-8"
	}

	// If detected charset is UTF-8, return buffered reader with no changes
	if strings.EqualFold(detectedCharset, "utf-8") {
		return bufferedReader, nil
	}

	e, err := htmlindex.Get(detectedCharset)
	if err != nil {
		// If we can't get encoding, default to UTF-8 and return the original body which is already
		// valid UTF-8.
		slog.Warn("Unknown charset", "charset", detectedCharset, "error", err)
		return body, nil
	}
	decoder := e.NewDecoder()
	return transform.NewReader(bufferedReader, decoder), nil
}

func getCharsetFromContentType(contentType string) string {
	if contentType == "" {
		return ""
	}
	for _, param := range strings.Split(contentType, ";") {
		param = strings.TrimSpace(param)
		if strings.HasPrefix(param, "charset=") {
			return strings.TrimPrefix(param, "charset=")
		}
	}
	return ""
}

func (u *URL) GetDocument() (doc *goquery.Document, err error) {
	u.documentMu.Lock()
	defer u.documentMu.Unlock()

	if u.document == nil {
		resp := u.GetResponse()
		if resp == nil {
			return nil, fmt.Errorf("response is nil for URL: %s", u.Raw)
		}
		err = u.RewindBody()
		if err != nil {
			return nil, err
		}
		contentType := resp.Header.Get("Content-Type")

		// Use the determineCharset function. The ignored value can be used
		// to debug the determined charset.
		_, bodyReader, err := u.determineCharset(contentType)
		if err != nil {
			slog.Warn("failed to determine charset, proceeding with original body",
				"error", err,
				"url", u.Raw,
				"content-type", contentType)
			err = u.RewindBody()
			if err != nil {
				return nil, err
			}
			bodyReader = u.GetBody()
		}

		// Create the document from the properly decoded reader
		u.document, err = goquery.NewDocumentFromReader(bodyReader)
		if err != nil {
			return nil, err
		}

		// Rewind body again after parsing
		err = u.RewindBody()
		if err != nil {
			return nil, fmt.Errorf("could not rewind body after parsing: %w", err)
		}
	}

	return u.document, nil
}

func (u *URL) determineCharset(contentType string) (string, io.Reader, error) {
	bodyBytes, err := io.ReadAll(u.GetBody())
	if err != nil {
		return "", nil, err
	}

	// Parse HTML to find meta tags first
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyBytes))
	if err != nil {
		// If parsing fails, fall back to header (or utf-8)
		return u.determineCharsetFallback(contentType, bodyBytes)
	}

	var metaCharset string
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		charsetVal, exists := s.Attr("charset")
		if exists {
			metaCharset = charsetVal
			return
		}

		httpEquiv, httpExists := s.Attr("http-equiv")
		content, contentExists := s.Attr("content")

		if httpExists && strings.ToLower(httpEquiv) == "content-type" && contentExists {
			if strings.Contains(content, "charset=") {
				metaCharset = strings.TrimSpace(strings.TrimPrefix(content, "text/html; charset="))
				return
			}
		}
	})

	if metaCharset != "" {
		decodedReader, err := createDecodedReader(bytes.NewReader(bodyBytes), "text/html; charset="+metaCharset)
		if err != nil {
			return "", bytes.NewReader(bodyBytes), err
		}
		return metaCharset, decodedReader, nil
	}

	// If no meta charset found, fall back to header (or UTF-8)
	return u.determineCharsetFallback(contentType, bodyBytes)

}

func (u *URL) determineCharsetFallback(contentType string, bodyBytes []byte) (string, io.Reader, error) {
	headerCharset := getCharsetFromContentType(contentType)
	if headerCharset != "" {
		decodedReader, err := createDecodedReader(bytes.NewReader(bodyBytes), contentType)
		if err != nil {
			return "", bytes.NewReader(bodyBytes), err
		}
		return headerCharset, decodedReader, nil
	}

	// If no header charset found, use createDecodedReader with UTF-8 as the fallback.
	decodedReader, err := createDecodedReader(bytes.NewReader(bodyBytes), "text/html; charset=utf-8")
	if err != nil {
		return "", bytes.NewReader(bodyBytes), err
	}
	return "utf-8", decodedReader, nil
}

func (u *URL) SetDocument(doc *goquery.Document) {
	u.document = doc
}

func (u *URL) GetMIMEType() *mimetype.MIME {
	return u.mimetype
}

func (u *URL) SetMIMEType(mimetype *mimetype.MIME) {
	u.mimetype = mimetype
}

func (u *URL) RewindBody() error {
	if u.body == nil {
		return fmt.Errorf("body is nil")
	}
	_, err := u.body.Seek(0, io.SeekStart)
	if err != nil {
		slog.Warn("failed to rewind body", "error", err)
	}
	return err
}

func (u *URL) SetRequest(r *http.Request) {
	u.request = r
}

func (u *URL) GetRequest() *http.Request {
	return u.request
}

func (u *URL) GetParsed() *url.URL {
	return u.parsed
}

func (u *URL) SetResponse(r *http.Response) {
	u.response = r
}

func (u *URL) GetResponse() *http.Response {
	return u.response
}

func (u *URL) GetRedirects() int {
	return u.Redirects
}

func (u *URL) IncRedirects() {
	u.Redirects++
}

func (u *URL) SetHops(hops int) {
	u.Hops = hops
}

func (u *URL) GetHops() int {
	return u.Hops
}

func (u *URL) String() string {
	u.once.Do(func() {
		u.stringCache = URLToString(u.parsed)
	})
	return u.stringCache
}

// URLToString exists to apply some custom stuff, in opposition of simply
// using the u.parsed.String() method
func URLToString(URL *url.URL) string {

	// Clone the URL to avoid modifying the original
	urlCopy := *URL

	switch urlCopy.Host {
	case "external-preview.redd.it", "styles.redditmedia.com", "preview.redd.it":
		// Do nothing. We don't want to encode the URL for signature purposes. :(
		break
	default:
		urlCopy.RawQuery = encodeQuery(urlCopy.Query())
		if strings.Contains(urlCopy.Host, ":") {
			hostWithoutPort, port, err := net.SplitHostPort(urlCopy.Host)
			if err != nil {
				slog.Warn("cannot split host and port", "error", err)
			} else {
				urlCopy.Host = hostWithoutPort + ":" + port
			}
		}
	}

	return urlCopy.String()
}

// Encode encodes the values into “URL encoded” form
// from: https://cs.opensource.google/go/go/+/refs/tags/go1.23.1:src/net/url/url.go;l=1002
// REASON: it has been modified to not sort
func encodeQuery(v url.Values) string {
	if len(v) == 0 {
		return ""
	}

	var buf strings.Builder

	first := true

	for k, vs := range v {
		keyEscaped := url.QueryEscape(k)
		for _, v := range vs {
			if !first {
				buf.WriteByte('&')
			}

			first = false

			buf.WriteString(keyEscaped)
			buf.WriteByte('=')
			buf.WriteString(url.QueryEscape(v))
		}
	}

	return buf.String()
}
