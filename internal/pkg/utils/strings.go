package utils

import (
	"net/url"
	"strings"
)

// StringSliceToURLSlice takes a slice of string and return a slice of url.URL
func StringSliceToURLSlice(rawURLs []string) (URLs []*url.URL) {
	for _, URL := range rawURLs {
		URL, err := url.Parse(URL)
		if err != nil {
			continue
		}

		URLs = append(URLs, URL)
	}

	return URLs
}

// StringContainsSliceElements if the string contains one of the elements
// of a slice
func StringContainsSliceElements(target string, slice []string) bool {
	for _, elem := range slice {
		if strings.Contains(target, elem) {
			return true
		}
	}
	return false
}

// DedupeStrings take a slice of string and dedupe it
func DedupeStrings(input []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range input {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
