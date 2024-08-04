package utils

import (
	"errors"
	"fmt"
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
	u.RawQuery = q.Encode()
	u.Host, err = idna.ToASCII(u.Host)
	if err != nil {
		if strings.Contains(u.Host, ":") {
			hostWithoutPort, _, _ := net.SplitHostPort(u.Host)
			asciiHost, err := idna.ToASCII(hostWithoutPort)
			if err == nil {
				u.Host = asciiHost + ":" + u.Port()
			} else {
				slog.Warn(fmt.Sprintf("could not IDNA encode URL: %s", err))
			}
		} else {
			slog.Warn(fmt.Sprintf("could not IDNA encode URL: %s", err))
		}
	}

	tempHost, err := idna.ToASCII(u.Hostname())
	if err != nil {
		slog.Warn(fmt.Sprintf("could not IDNA encode URL: %s", err))
		tempHost = u.Hostname()
	}

	if strings.Contains(tempHost, ":") && !(strings.HasPrefix(tempHost, "[") && strings.HasSuffix(tempHost, "]")) {
		tempHost = "[" + tempHost + "]"
	}

	port := u.Port()
	if len(port) > 0 {
		u.Host = tempHost + ":" + port
	} else {
		u.Host = tempHost
	}

	return u.String()
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
