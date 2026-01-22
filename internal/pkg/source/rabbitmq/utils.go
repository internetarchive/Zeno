package hqr

import (
	"strings"
)

func pathToHops(path string) (hops int) {
	// For each L in the path, add 1 hop
	return strings.Count(path, "L")
}

func hopsToPath(hops int) (path string) {
	// For each hop, add an L to the path
	return strings.Repeat("L", hops)
}
