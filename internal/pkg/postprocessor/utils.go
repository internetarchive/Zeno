package postprocessor

func isStatusCodeRedirect(statusCode int) bool {
	switch statusCode {
	case 300, 301, 302, 303, 307, 308:
		return true
	default:
		return false
	}
}
