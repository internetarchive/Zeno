package domainscrawl

import "testing"

func TestART_InsertAndExactMatch(t *testing.T) {
	t.Parallel()

	a := newART()
	a.Insert("example.com")
	a.Insert("sub.example.org")

	if !a.ExactMatch("example.com") {
		t.Fatalf("ExactMatch(example.com) = false; want true")
	}
	if !a.ExactMatch("sub.example.org") {
		t.Fatalf("ExactMatch(sub.example.org) = false; want true")
	}
	if a.ExactMatch("EXAMPLE.COM") { // current behavior: case-sensitive exact map
		t.Fatalf("ExactMatch(EXAMPLE.COM) = true; want false (case-sensitive)")
	}
}

func TestART_PrefixMatch_SubdomainAndExact(t *testing.T) {
	t.Parallel()

	a := newART()
	a.Insert("example.com")

	// exact-as-prefix
	if !a.PrefixMatch("example.com") {
		t.Fatalf("PrefixMatch(example.com) = false; want true")
	}
	// subdomain
	if !a.PrefixMatch("api.example.com") {
		t.Fatalf("PrefixMatch(api.example.com) = false; want true")
	}
	// deeper subdomain
	if !a.PrefixMatch("x.y.example.com") {
		t.Fatalf("PrefixMatch(x.y.example.com) = false; want true")
	}
}

func TestART_PrefixMatch_NoMatch(t *testing.T) {
	t.Parallel()

	a := newART()
	a.Insert("example.com")
	a.Insert("foo.bar")

	if a.PrefixMatch("example.org") {
		t.Fatalf("PrefixMatch(example.org) = true; want false")
	}
	if a.PrefixMatch("bar.foo") {
		t.Fatalf("PrefixMatch(bar.foo) = true; want false (labels reversed domain)")
	}
	if a.PrefixMatch("localhost") {
		t.Fatalf("PrefixMatch(localhost) = true; want false (single label)")
	}
}

func TestNormalizeDomain_Basics(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in, want string
	}{
		{"Example.COM.", "example.com"},
		{"example.com", "example.com"},
		{"LOCALHOST", "localhost"},
		{".", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeDomain(c.in); got != c.want {
			t.Fatalf("normalizeDomain(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}
