package utils

import (
	"net/url"
	"strings"

	"golang.org/x/net/idna"
)

func URLToString(u *url.URL) string {
	var err error

	q := u.Query()
	u.RawQuery = q.Encode()
	u.Host, err = idna.ToASCII(u.Host)
	if err != nil {
		LogWarning.Warningf("could not IDNA encode URL: %s", err)
	}

	tempHost, err := idna.ToASCII(u.Hostname())
	if err != nil {
		LogWarning.Warningf("could not IDNA encode URL: %s", err)
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
