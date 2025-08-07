package extractor

// Check if the rune is a ascii newline (\n or \r).
func isNewline(c rune) bool {
	return c == '\n' || c == '\r'
}

// isWhitespace returns true for space, \n, \r, \t, \f.
func isWhitespace(c rune) bool {
	return c == ' ' || c == '\n' || c == '\r' || c == '\t' || c == '\f'
}

// similar to strings.EqualFold, but works with runes.
func equalFold(s, t []rune) bool {
	if len(s) != len(t) {
		return false
	}
	i := 0
	for ; i < len(s) && i < len(t); i++ {
		sr := s[i]
		tr := t[i]

		// Easy case.
		if tr == sr {
			continue
		}

		// Make sr < tr to simplify what follows.
		if tr < sr {
			tr, sr = sr, tr
		}

		// ASCII only, sr/tr must be upper/lower case
		if sr >= 'A' && sr <= 'Z' && tr == sr+'a'-'A' {
			continue
		}
		return false

	}
	return true
}

// similar to strings.HasPrefixFold, but works with runes.
func hasPrefixFold(s, t []rune) bool {
	if len(s) < len(t) {
		return false
	}
	return equalFold(s[:len(t)], t)
}
