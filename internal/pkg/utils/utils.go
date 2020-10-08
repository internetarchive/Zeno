package utils

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"sync/atomic"
	"time"

	"github.com/asaskevich/govalidator"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
)

// TAtomBool define an atomic boolean
type TAtomBool struct{ flag int32 }

// Set set the value of an atomic boolean
func (b *TAtomBool) Set(value bool) {
	var i int32 = 0
	if value {
		i = 1
	}
	atomic.StoreInt32(&(b.flag), int32(i))
}

// Get return the value of an atomic boolean
func (b *TAtomBool) Get() bool {
	if atomic.LoadInt32(&(b.flag)) != 0 {
		return true
	}
	return false
}

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

// DedupeURLs take a slice of *url.URL and dedupe it
func DedupeURLs(URLs []url.URL) []url.URL {
	keys := make(map[string]bool)
	list := []url.URL{}
	for _, entry := range URLs {
		if _, value := keys[entry.String()]; !value {
			keys[entry.String()] = true
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

func SetupLogging(jobPath string) *logrus.Logger {
	var logsDirectory = path.Join(jobPath, "logs")
	var log = logrus.New()

	// Create logs directory for the job
	os.MkdirAll(logsDirectory, os.ModePerm)

	path := path.Join(logsDirectory, "zeno")
	writer, err := rotatelogs.New(
		fmt.Sprintf("%s_%s.log", path, "%Y%m%d%H%M%S"),
		rotatelogs.WithRotationTime(time.Hour*6),
	)
	if err != nil {
		logrus.Fatalf("Failed to Initialize Log File %s", err)
	}
	log.SetOutput(writer)

	return log
}
