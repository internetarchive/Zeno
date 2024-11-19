package hq

func pathToHop(path string) (hop int) {
	// For each L in the path, add 1 hop
	for _, c := range path {
		if c == 'L' {
			hop++
		}
	}

	return hop
}
