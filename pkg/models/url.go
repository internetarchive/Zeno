package models

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/gabriel-vasile/mimetype"
	"github.com/internetarchive/gowarc/pkg/spooledtempfile"
	"golang.org/x/net/idna"
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

func (u *URL) GetDocument() (doc *goquery.Document, err error) {
	if u.document == nil {
		u.document, err = goquery.NewDocumentFromReader(u.GetBody())
		if err != nil {
			return nil, err
		}

		u.RewindBody()
	}

	return u.document, nil
}

func (u *URL) SetDocument(doc *goquery.Document) {
	u.document = doc
}

// if mimetype is not set, try to get it from Content-Type header and cache it.
func (u *URL) GetMIMEType() *mimetype.MIME {
	if u.mimetype != nil {
		return u.mimetype
	}
	if u.GetResponse() != nil {
		ct := u.GetResponse().Header.Get("Content-Type")
		if ct != "" {
			mt := strings.Split(ct, ";")[0]
			mt = strings.TrimSpace(mt)
			mt = strings.ToLower(mt)
			u.mimetype = mimetype.Lookup(mt)
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
