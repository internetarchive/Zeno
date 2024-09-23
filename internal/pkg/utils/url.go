package utils

import (
	"errors"
	"log/slog"
	"net"
	"net/url"
	"strings"

	"github.com/asaskevich/govalidator"
	"golang.org/x/net/idna"
)

func URLToString(u *url.URL) string {
	var err error

	q := u.Query()
	u.RawQuery = ZenoEncode(q)
	u.Host, err = idna.ToASCII(u.Host)
	if err != nil {
		if strings.Contains(u.Host, ":") {
			hostWithoutPort, port, err := net.SplitHostPort(u.Host)
			if err != nil {
				slog.Warn("cannot split host and port", "error", err)
			} else {
				asciiHost, err := idna.ToASCII(hostWithoutPort)
				if err == nil {
					u.Host = asciiHost + ":" + port
				} else {
					slog.Warn("cannot encode punycode host without port to ASCII", "error", err)
				}
			}
		} else {
			slog.Warn("cannot encode punycode host to ASCII", "error", err)
		}
	}

	return u.String()
}

// Encode encodes the values into “URL encoded” form
// from: https://cs.opensource.google/go/go/+/refs/tags/go1.23.1:src/net/url/url.go;l=1002
// modified to not sort.
func ZenoEncode(v url.Values) string {
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

// MakeAbsolute turn all URLs in a slice of url.URL into absolute URLs, based
// on a given base *url.URL
func MakeAbsolute(base *url.URL, URLs []*url.URL) []*url.URL {
	for i, URL := range URLs {
		if !URL.IsAbs() {
			URLs[i] = base.ResolveReference(URL)
		}
	}

	return URLs
}

func RemoveFragments(URLs []*url.URL) []*url.URL {
	for i := range URLs {
		URLs[i].Fragment = ""
	}

	return URLs
}

// DedupeURLs take a slice of *url.URL and dedupe it
func DedupeURLs(URLs []*url.URL) []*url.URL {
	keys := make(map[string]bool)
	list := []*url.URL{}

	for _, entry := range URLs {
		if _, value := keys[URLToString(entry)]; !value {
			keys[URLToString(entry)] = true

			if entry.Scheme == "http" || entry.Scheme == "https" {
				list = append(list, entry)
			}
		}
	}

	return list
}

// ValidateURL validates a *url.URL
func ValidateURL(u *url.URL) error {
	valid := govalidator.IsURL(URLToString(u))

	if u.Scheme != "http" && u.Scheme != "https" {
		valid = false
	}

	if !valid {
		return errors.New("not a valid URL")
	}

	return nil
}
