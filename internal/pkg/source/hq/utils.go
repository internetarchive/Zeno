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

func hopsToPath(hops int) (path string) {
	// For each hop, add an L to the path
	for i := 0; i < hops; i++ {
		path += "L"
	}

	return path
}
