package models

import (
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/idna"
)

type URL struct {
	Raw     string
	parsed  *url.URL
	request *http.Request
	Hop     int // This determines the number of hops this item is the result of, a hop is a "jump" from 1 page to another page
}

func (u *URL) Parse() (err error) {
	u.parsed, err = url.ParseRequestURI(u.Raw)
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

func (u *URL) SetHop(hop int) {
	u.Hop = hop
}

func (u *URL) GetHop() int {
	return u.Hop
}

// String exists to apply some custom stuff, in opposition of simply
// using the u.parsed.String() method
func (u *URL) String() string {
	var err error

	switch u.parsed.Host {
	case "external-preview.redd.it", "styles.redditmedia.com", "preview.redd.it":
		// Do nothing. We don't want to encode the URL for signature purposes. :(
		break
	default:
		q := u.parsed.Query()
		u.parsed.RawQuery = encodeQuery(q)
	}
	u.parsed.Host, err = idna.ToASCII(u.parsed.Host)
	if err != nil {
		if strings.Contains(u.parsed.Host, ":") {
			hostWithoutPort, port, err := net.SplitHostPort(u.parsed.Host)
			if err != nil {
				slog.Warn("cannot split host and port", "error", err)
			} else {
				asciiHost, err := idna.ToASCII(hostWithoutPort)
				if err == nil {
					u.parsed.Host = asciiHost + ":" + port
				} else {
					slog.Warn("cannot encode punycode host without port to ASCII", "error", err)
				}
			}
		} else {
			slog.Warn("cannot encode punycode host to ASCII", "error", err)
		}
	}

	return u.parsed.String()
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
