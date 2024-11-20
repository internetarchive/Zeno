package hq

func pathToHops(path string) (hops int) {
	// For each L in the path, add 1 hop
	for _, c := range path {
		if c == 'L' {
			hops++
		}
	}

	return hops
}
