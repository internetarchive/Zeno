package domainscrawl

import (
	"strings"
	"sync"

	art "github.com/plar/go-adaptive-radix-tree/v2"
)

// We have a struct on top of the ART to allow for locking
type ART struct {
	sync.RWMutex
	exact map[string]struct{}
	tree  art.Tree
}

func newART() *ART {
	return &ART{
		RWMutex: sync.RWMutex{},
		exact:   make(map[string]struct{}),
		tree:    art.New(),
	}
}

func (a *ART) Size() int {
	a.RLock()
	defer a.RUnlock()

	return a.tree.Size()
}

func (a *ART) Insert(key string) {
	a.Lock()
	defer a.Unlock()

	// Insert the key in reverse order to allow for faster prefix matching
	a.tree.Insert(art.Key(reverseHost(key)), struct{}{})

	// Also store the exact key for faster lookups for exact matches
	// (O(1), map vs O(n), ART traversal)
	a.exact[key] = struct{}{}
}

func (a *ART) ExactMatch(key string) bool {
	a.RLock()
	defer a.RUnlock()
	_, found := a.exact[key]
	return found
}

// PrefixMatch returns true if ANY stored domain is a suffix of host
// (i.e., host is that domain or a subdomain). We do this by checking
// whether any prefix of the reversed host exists in the ART.
func (a *ART) PrefixMatch(host string) bool {
	a.RLock()
	defer a.RUnlock()

	rh := reverseHost(normalizeDomain(host)) // e.g. "sub.example.com" -> "com.example.sub"
	// Check cumulative prefixes: "com", "com.example", "com.example.sub"
	for i := 0; i < len(rh); {
		// advance to next dot or end to form rh[:end]
		j := strings.IndexByte(rh[i:], '.')
		end := len(rh)
		if j >= 0 {
			end = i + j
		}
		if _, found := a.tree.Search(art.Key(rh[:end])); found {
			return true
		}
		if j < 0 {
			break
		}
		i = end + 1 // skip the dot
	}
	return false
}

// Range returns all keys in the tree, mostly used for testing
func (a *ART) Range() []string {
	a.RLock()
	defer a.RUnlock()

	out := make([]string, 0, a.tree.Size())
	a.tree.ForEach(func(node art.Node) bool {
		out = append(out, reverseHost(string(node.Key())))
		return true
	})

	return out
}

func normalizeDomain(s string) string {
	return strings.TrimSuffix(strings.ToLower(s), ".")
}
