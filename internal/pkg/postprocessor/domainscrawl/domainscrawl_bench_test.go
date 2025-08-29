package domainscrawl

import (
	"fmt"
	"testing"
)

// sink variables to avoid dead-code elimination in benchmarks
var sinkBool bool

// helper to build many naive domains like "example1234.com"
func buildNaiveDomains(n int) []string {
	out := make([]string, 0, n)
	for i := range n {
		out = append(out, fmt.Sprintf("example%d.com", i))
	}
	return out
}

// -----------------------------
// ART.ExactMatch benchmarks
// -----------------------------

func BenchmarkART_ExactMatch(b *testing.B) {
	sizes := []int{1_000, 10_000, 50_000}
	for _, n := range sizes {
		b.Run(fmt.Sprintf("N=%d_hit", n), func(b *testing.B) {
			Reset()
			// Insert directly into ART (no URL parsing)
			domains := buildNaiveDomains(n)
			for _, d := range domains {
				globalMatcher.domains.Insert(d)
			}
			needle := domains[n/2]

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if globalMatcher.domains.ExactMatch(needle) {
					sinkBool = !sinkBool
				}
			}
		})
		b.Run(fmt.Sprintf("N=%d_miss", n), func(b *testing.B) {
			Reset()
			domains := buildNaiveDomains(n)
			for _, d := range domains {
				globalMatcher.domains.Insert(d)
			}
			needle := "not-in-set.com"

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if globalMatcher.domains.ExactMatch(needle) {
					sinkBool = !sinkBool
				}
			}
		})
	}
}

func BenchmarkART_ExactMatch_Parallel(b *testing.B) {
	n := 50_000
	Reset()
	domains := buildNaiveDomains(n)
	for _, d := range domains {
		globalMatcher.domains.Insert(d)
	}
	needle := domains[n/2]

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if globalMatcher.domains.ExactMatch(needle) {
				sinkBool = !sinkBool
			}
		}
	})
}

// -----------------------------
// ART.PrefixMatch benchmarks
// -----------------------------

func BenchmarkART_PrefixMatch(b *testing.B) {
	sizes := []int{1_000, 10_000, 50_000}
	for _, n := range sizes {
		b.Run(fmt.Sprintf("N=%d_hit", n), func(b *testing.B) {
			Reset()
			domains := buildNaiveDomains(n)
			for _, d := range domains {
				globalMatcher.domains.Insert(d)
			}
			host := "sub." + domains[n/2] // subdomain -> should match

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if globalMatcher.domains.PrefixMatch(host) {
					sinkBool = !sinkBool
				}
			}
		})
		b.Run(fmt.Sprintf("N=%d_miss", n), func(b *testing.B) {
			Reset()
			domains := buildNaiveDomains(n)
			for _, d := range domains {
				globalMatcher.domains.Insert(d)
			}
			host := "sub.not-in-set.com" // should miss

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if globalMatcher.domains.PrefixMatch(host) {
					sinkBool = !sinkBool
				}
			}
		})
	}
}

func BenchmarkART_PrefixMatch_Parallel(b *testing.B) {
	n := 50_000
	Reset()
	domains := buildNaiveDomains(n)
	for _, d := range domains {
		globalMatcher.domains.Insert(d)
	}
	host := "deep.sub." + domains[n/2]

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if globalMatcher.domains.PrefixMatch(host) {
				sinkBool = !sinkBool
			}
		}
	})
}

// -----------------------------
// Match() benchmarks (naive domains)
// -----------------------------

func BenchmarkMatch_NaiveDomain_Exact(b *testing.B) {
	n := 50_000
	Reset()

	// Build elements once through AddElements so globalMatcher is populated consistently
	elements := buildNaiveDomains(n)
	if err := AddElements(elements, nil); err != nil {
		b.Fatalf("AddElements: %v", err)
	}
	raw := "https://" + elements[n/2] // exact domain hit

	b.ReportAllocs()

	for b.Loop() {
		if Match(raw) {
			sinkBool = !sinkBool
		}
	}
}

func BenchmarkMatch_NaiveDomain_Subdomain(b *testing.B) {
	n := 50_000
	Reset()

	elements := buildNaiveDomains(n)
	if err := AddElements(elements, nil); err != nil {
		b.Fatalf("AddElements: %v", err)
	}
	raw := "https://sub." + elements[n/2] + "/path?q=1" // subdomain hit via PrefixMatch

	b.ReportAllocs()

	for b.Loop() {
		if Match(raw) {
			sinkBool = !sinkBool
		}
	}
}

func BenchmarkMatch_NaiveDomain_Miss(b *testing.B) {
	n := 50_000
	Reset()

	elements := buildNaiveDomains(n)
	if err := AddElements(elements, nil); err != nil {
		b.Fatalf("AddElements: %v", err)
	}
	raw := "https://sub.not-in-set.com/path"

	b.ReportAllocs()

	for b.Loop() {
		if Match(raw) {
			sinkBool = !sinkBool
		}
	}
}

func BenchmarkMatch_NaiveDomain_Subdomain_Parallel(b *testing.B) {
	n := 50_000
	Reset()

	elements := buildNaiveDomains(n)
	if err := AddElements(elements, nil); err != nil {
		b.Fatalf("AddElements: %v", err)
	}
	raw := "https://deep.sub." + elements[n/2]

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if Match(raw) {
				sinkBool = !sinkBool
			}
		}
	})
}

// -----------------------------
// Optional: full-URL exact & regex
// -----------------------------

func BenchmarkMatch_FullURL_Exact(b *testing.B) {
	Reset()
	el := []string{"https://foo.example.com/path?x=1", "https://bar.example.com/"}
	if err := AddElements(el, nil); err != nil {
		b.Fatalf("AddElements: %v", err)
	}
	raw := "https://foo.example.com/path?x=1"

	b.ReportAllocs()

	for b.Loop() {
		if Match(raw) {
			sinkBool = !sinkBool
		}
	}
}

func BenchmarkMatch_Regex(b *testing.B) {
	Reset()
	el := []string{`^https?://([a-z0-9-]+\.)*example\.net/.*$`}
	if err := AddElements(el, nil); err != nil {
		b.Fatalf("AddElements: %v", err)
	}
	rawHit := "https://a.b.c.example.net/path"
	rawMiss := "https://not.example.org/"

	b.ReportAllocs()

	for b.Loop() {
		if Match(rawHit) {
			sinkBool = !sinkBool
		}
		if Match(rawMiss) {
			sinkBool = !sinkBool
		}
	}
}
