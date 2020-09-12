package utils

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"net/url"

	"github.com/asaskevich/govalidator"
)

// GetSHA1 take a string and return the SHA1 hash of the string, as a string
func GetSHA1(str string) string {
	hash := sha1.New()
	hash.Write([]byte(str))
	return hex.EncodeToString(hash.Sum(nil))
}

// StringInSlice return true if the string is in the slice
func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

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
