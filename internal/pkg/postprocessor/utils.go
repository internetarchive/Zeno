package postprocessor

import (
	"regexp"

	"github.com/internetarchive/Zeno/pkg/models"
)

var (
	skipProtocolsRe = regexp.MustCompile(`(?i)^(data|file|javascript|mailto|sms|tel):`)
)

func isStatusCodeRedirect(statusCode int) bool {
	switch statusCode {
	case 300, 301, 302, 303, 307, 308:
		return true
	default:
		return false
	}
}

func filterURLsByProtocol(links []*models.URL) []*models.URL {
	var filtered []*models.URL
	for _, link := range links {
		if !skipProtocolsRe.MatchString(link.Raw) {
			filtered = append(filtered, link)
		}
	}
	return filtered
}
