package domainscrawl

import (
	"testing"
)

// Table-driven unit tests that cover many scenarios.
func TestReverseHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		// Basic / multi-label
		{"basic_3_labels", "www.google.com", "com.google.www"},
		{"basic_5_labels", "a.b.c.d.e", "e.d.c.b.a"},
		{"two_labels", "example.com", "com.example"},
		{"single_label_localhost", "localhost", "localhost"},

		// Trailing dot & case-normalization
		{"trailing_dot", "example.com.", "com.example"},
		{"uppercased", "WWW.GOOGLE.COM", "com.google.www"},
		{"mixed_case", "Sub.ExAmPlE.CoM", "com.example.sub"},

		// Ports
		{"port_https", "www.google.com:443", "com.google.www:443"},
		{"port_http", "example.com:80", "com.example:80"},
		{"port_custom", "svc.env.example.org:8443", "org.example.env.svc:8443"},

		// IPv4 (unchanged)
		{"ipv4_plain", "127.0.0.1", "127.0.0.1"},
		{"ipv4_with_port", "127.0.0.1:8080", "127.0.0.1:8080"},

		// IPv6 with brackets (unchanged)
		{"ipv6_bracketed", "[2001:db8::1]:443", "[2001:db8::1]:443"},
		{"ipv6_loopback", "[::1]:80", "[::1]:80"},

		// Punycode (ASCII domain label form)
		{"punycode_label", "www.xn--bcher-kva.example", "example.xn--bcher-kva.www"},
		{"punycode_root", "xn--fsqu00a.xn--0zwm56d", "xn--0zwm56d.xn--fsqu00a"},

		// Raw Unicode IDN input (URLs should be ASCII, but we handle bytes & lower-casing)
		// Note: behavior is "best effort" unless we normalize with x/net/idna first.
		{"unicode_idn", "www.bücher.example", "example.bücher.www"},

		// Many colons but not IPv6
		{"many_colons_not_ipv6", "a:b:c.example.com", "com.example.a:b:c"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := reverseHost(tt.in)
			if got != tt.want {
				t.Fatalf("reverseHost(%q) = %q; want %q", tt.in, got, tt.want)
			}
		})
	}
}

func BenchmarkReverseHost(b *testing.B) {
	inputs := []string{
		"www.google.com",
		"svc.env.namespace.cluster.example.org",
		"EXAMPLE.COM.",
		"localhost",
		"127.0.0.1",
		"[2001:db8::1]:443",
		"www.xn--bcher-kva.example",
		"www.bücher.example",
		"a..b.example.com",
		"example.com:8443",
		"example.com:abc",   // invalid port (current behavior)
		"a:b:c.example.com", // many colons but not IPv6
	}

	b.ReportAllocs()
	for b.Loop() {
		for _, in := range inputs {
			_ = reverseHost(in)
		}
	}
}
