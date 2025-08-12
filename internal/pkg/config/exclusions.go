package config

import (
	"bufio"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

func (c *Config) GetExclusionRegexes() []*regexp.Regexp {
	return c.exclusionRegexes.Load().([]*regexp.Regexp)
}

func (c *Config) setExclusionRegexes(next []*regexp.Regexp) {
	c.exclusionRegexes.Store(next)
}

func (c *Config) loadExclusions(file string) ([]*regexp.Regexp, error) {
	var (
		regexes []string
		err     error
	)

	if strings.HasPrefix(file, "http://") || strings.HasPrefix(file, "https://") {
		slog.Info("reading (remote) exclusion file", "file", file)
		regexes, err = c.readRemoteExclusionFile(file)
		if err != nil {
			return nil, err
		}
	} else {
		slog.Info("reading (local) exclusion file", "file", file)
		regexes, err = readLocalExclusionFile(file)
		if err != nil {
			return nil, err
		}
	}

	slog.Info("compiling exclusion regexes", "regexes", len(regexes))

	return compileRegexes(regexes), nil
}

func (c *Config) readRemoteExclusionFile(URL string) (regexes []string, err error) {
	httpClient := &http.Client{
		Timeout: time.Second * 5,
	}

	req, err := http.NewRequest(http.MethodGet, URL, nil)
	if err != nil {
		return regexes, err
	}

	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return regexes, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return regexes, fmt.Errorf("failed to download exclusion file: %s", resp.Status)
	}

	// Read file line by line
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		regexes = append(regexes, scanner.Text())
	}
	return regexes, scanner.Err()
}

func compileRegexes(regexes []string) []*regexp.Regexp {
	var compiledRegexes []*regexp.Regexp

	for _, regex := range regexes {
		slog.Debug("compiling regex", "regex", regex)
		compiledRegex := regexp.MustCompile(regex)

		compiledRegexes = append(compiledRegexes, compiledRegex)
	}

	return compiledRegexes
}

func readLocalExclusionFile(file string) (regexes []string, err error) {
	f, err := os.Open(file)
	if err != nil {
		return regexes, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		regexes = append(regexes, scanner.Text())
	}

	return regexes, scanner.Err()
}
