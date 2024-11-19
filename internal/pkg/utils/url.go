package utils

import (
	"github.com/internetarchive/Zeno/pkg/models"
)

// DedupeURLs take a slice of *url.URL and dedupe it
func DedupeURLs(URLs []*models.URL) []*models.URL {
	keys := make(map[string]bool)
	list := make([]*models.URL, 0, len(URLs))

	for _, entry := range URLs {
		if _, value := keys[entry.String()]; !value {
			keys[entry.String()] = true

			if entry.Parsed().Scheme == "http" || entry.Parsed().Scheme == "https" {
				list = append(list, entry)
			}
		}
	}

	return list
}
