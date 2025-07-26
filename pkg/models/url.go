package models

import (
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/gabriel-vasile/mimetype"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
	"golang.org/x/net/idna"
	"golang.org/x/text/encoding"
)

type URL struct {
	Raw       string
	parsed    *url.URL
	request   *http.Request
	response  *http.Response
	base      *url.URL // Base is the base URL of the HTML doc, extracted from a <base> tag
	body      spooledtempfile.ReadSeekCloser
	mimetype  *mimetype.MIME
	Hops      int // This determines the number of hops this item is the result of, a hop is a "jump" from 1 page to another page
	Redirects int

	stringCache string
	once        sync.Once

	documentCache        *goquery.Document // Transformed utf8 document in-memory cache
	documentEncoding     encoding.Encoding // Encoding of the document
	DocumentTransfromMux sync.Mutex        // Protect document transform

}

// NewURL parses a raw URL string and returns a URL object.
// If the URL is invalid, it returns a URL object with the raw string and an error.
func NewURL(raw string) (URL, error) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return URL{
			Raw: raw,
		}, err
	}
	return URL{
		Raw:    raw,
		parsed: parsed,
	}, nil
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

func (u *URL) GetDocumentCache() *goquery.Document {
	return u.documentCache
}

func (u *URL) SetDocumentCache(doc *goquery.Document) {
	u.documentCache = doc
}

func (u *URL) GetDocumentEncoding() encoding.Encoding {
	return u.documentEncoding
}

func (u *URL) SetDocumentEncoding(enc encoding.Encoding) {
	u.documentEncoding = enc
}

// if mimetype is not set, try to get it from Content-Type header and cache it.
func (u *URL) GetMIMEType() *mimetype.MIME {
	if u.mimetype != nil {
		return u.mimetype
	}
	if u.GetResponse() != nil {
		ct := u.GetResponse().Header.Get("Content-Type")
		if ct != "" {
			mt, _, err := mime.ParseMediaType(strings.TrimSpace(ct))
			if err == nil {
				extendMimetype()
				u.mimetype = mimetype.Lookup(mt)
			}
		}
	}
	return u.mimetype
}

func (u *URL) SetMIMEType(mimetype *mimetype.MIME) {
	u.mimetype = mimetype
}

func (u *URL) RewindBody() {
	_, err := u.body.Seek(0, io.SeekStart)
	if err != nil {
		panic(err)
	}
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

// GetBase returns the base URL of the item
func (u *URL) GetBase() *url.URL {
	return u.base
}

// SetBase sets the base URL of the item
func (u *URL) SetBase(base *url.URL) {
	u.base = base
}

// URLToString exists to apply some custom stuff, in opposition of simply
// using the u.parsed.String() method
func URLToString(URL *url.URL) string {
	var err error

	switch URL.Host {
	case "external-preview.redd.it", "styles.redditmedia.com", "preview.redd.it":
		// Do nothing. We don't want to encode the URL for signature purposes. :(
		break
	default:
		URL.RawQuery = encodeQuery(URL.Query())
	}

	URL.Host, err = idna.ToASCII(URL.Host)
	if err != nil {
		if strings.Contains(URL.Host, ":") {
			hostWithoutPort, port, err := net.SplitHostPort(URL.Host)
			if err != nil {
				slog.Warn("cannot split host and port", "error", err)
			} else {
				hostWithoutPort, err = idna.ToASCII(hostWithoutPort)
				if err == nil {
					URL.Host = hostWithoutPort + ":" + port
				} else {
					slog.Warn("cannot encode punycode host without port to ASCII", "error", err)
				}
			}
		} else {
			slog.Warn("cannot encode punycode host to ASCII", "error", err)
		}
	}

	return URL.String()
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
