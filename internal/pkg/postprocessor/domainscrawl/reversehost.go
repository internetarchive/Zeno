package domainscrawl

import (
	"net"
	"strings"
)

// reverseHost turns "www.google.com" -> "com.google.www".
// It preserves ports, ignores trailing dots, normalizes to lower case,
// and leaves IPs (v4/v6) unchanged.
func reverseHost(hostport string) string {
	host := hostport
	port := ""

	// Split host:port if present (handles [::1]:443). If no port, keep as-is.
	if h, p, err := net.SplitHostPort(hostport); err == nil {
		host, port = h, p
	}

	// For URLs, IPv6 literals should be bracketed; but if it's an IP (v4/v6), don't touch.
	trimmed := strings.TrimSuffix(strings.ToLower(host), ".")
	if ip := net.ParseIP(trimmed); ip != nil {
		if port != "" {
			return net.JoinHostPort(host, port) // keep original host casing/brackets
		}
		return host
	}

	// Reverse labels without extra slice allocations.
	// (Works on ASCII/punycode; domains in URLs are ASCII after IDNA.)
	var b strings.Builder
	b.Grow(len(trimmed))

	i := len(trimmed)
	first := true
	for i > 0 {
		j := strings.LastIndexByte(trimmed[:i], '.')
		if !first {
			b.WriteByte('.')
		}
		first = false
		if j == -1 {
			b.WriteString(trimmed[:i])
			break
		}
		b.WriteString(trimmed[j+1 : i])
		i = j
	}

	out := b.String()
	if port != "" {
		return out + ":" + port
	}
	return out
}
