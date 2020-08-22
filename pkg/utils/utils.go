package utils

import (
	"errors"
	"net/url"

	"github.com/asaskevich/govalidator"
)

// DedupeStringSlice take a slice of string and dedupe it
func DedupeStringSlice(stringSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range stringSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// ValidateURL validates a *url.URL
func ValidateURL(u *url.URL) error {
	valid := govalidator.IsURL(u.String())

	if u.Scheme != "http" && u.Scheme != "https" {
		valid = false
	}

	if valid == false {
		return errors.New("Not a valid URL")
	}

	return nil
}
