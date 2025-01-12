package models

import (
	"bytes"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gabriel-vasile/mimetype"
	"golang.org/x/net/idna"
)

func init() {
	mimetype.SetLimit(0)
}

type URL struct {
	Raw       string
	parsed    *url.URL
	request   *http.Request
	response  *http.Response
	body      *bytes.Reader
	mimetype  *mimetype.MIME
	document  *goquery.Document
	Hops      int // This determines the number of hops this item is the result of, a hop is a "jump" from 1 page to another page
	Redirects int
}

func (u *URL) Parse() (err error) {
	u.parsed, err = url.ParseRequestURI(u.Raw)
	return err
}

func (u *URL) GetBody() *bytes.Reader {
	return u.body
}

func (u *URL) SetBody(body *bytes.Reader) {
	u.body = body
}

func (u *URL) ProcessBody() (err error) {
	// Consume the response body, this serves as a way to consume the body to write it in the WARC file,
	// but also to detect the MIME type of the body. If the mime type is one that we post-process, we will
	// store the body, else we will discard it.
	tempBody := bytes.NewBuffer(nil)
	_, err = io.Copy(tempBody, u.GetResponse().Body)
	if err != nil {
		return err
	}
	u.GetResponse().Body.Close()

	// Turn the body into a reader so that it can be read again
	u.SetBody(bytes.NewReader(tempBody.Bytes()))

	// Destroy the body buffer, we don't need it anymore
	tempBody = nil

	// We do not use http.DetectContentType because it only supports
	// a limited number of MIME types, those commonly found in web.
	u.mimetype, err = mimetype.DetectReader(u.GetBody())
	if err != nil {
		return err
	}
	u.RewindBody()

	// Check if the MIME type is one that we post-process
	if u.mimetype.Parent() != nil {
		switch u.mimetype.Parent().String() {
		case "text/plain":
			// Also create the goquery document
			u.document, err = goquery.NewDocumentFromReader(u.GetBody())
			if err != nil {
				return err
			}
			u.RewindBody()

			return nil
		}
	}

	// Set the URL body to nil, the mimetype is not one that we post-process
	u.SetBody(nil)

	return nil
}

func (u *URL) GetMIME() *mimetype.MIME {
	return u.mimetype
}

func (u *URL) GetDocument() *goquery.Document {
	return u.document
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
	return URLToString(u.parsed)
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
		q := URL.Query()
		URL.RawQuery = encodeQuery(q)
	}
	URL.Host, err = idna.ToASCII(URL.Host)
	if err != nil {
		if strings.Contains(URL.Host, ":") {
			hostWithoutPort, port, err := net.SplitHostPort(URL.Host)
			if err != nil {
				slog.Warn("cannot split host and port", "error", err)
			} else {
				asciiHost, err := idna.ToASCII(hostWithoutPort)
				if err == nil {
					URL.Host = asciiHost + ":" + port
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
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	// Modified to not sort the keys.
	// slices.Sort(keys)
	for _, k := range keys {
		vs := v[k]
		keyEscaped := url.QueryEscape(k)
		for _, v := range vs {
			if buf.Len() > 0 {
				buf.WriteByte('&')
			}
			buf.WriteString(keyEscaped)
			buf.WriteByte('=')
			buf.WriteString(url.QueryEscape(v))
		}
	}
	return buf.String()
}

// URLType qualifies the type of URL
type URLType string

const (
	// URLTypeSeed is for URLs that came from the queue or HQ
	URLTypeSeed URLType = "seed"
	// URLTypeRedirection is for URLs that are redirections
	URLTypeRedirection = "seed"
	// URLTypeAsset is for URLs that are assets of a page
	URLTypeAsset = "asset"
)
