package queue

import (
	"bufio"
	"errors"
	"net/url"
	"os"

	"github.com/CorentinB/Zeno/pkg/utils"
	log "github.com/sirupsen/logrus"
)

// IsSeedList validates if the path is a seed list, and return an array of the seeds if it is a seed list
func IsSeedList(path string) (seeds []Item, err error) {
	// Verify that the file exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// File doesn't exist
		return seeds, err
	}

	// Open the file
	file, err := os.Open(path)
	if err != nil {
		return seeds, err
	}
	defer file.Close()

	// Initialize scanner
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		URL, err := url.Parse(scanner.Text())
		if err != nil {
			log.WithFields(log.Fields{
				"url":   scanner.Text(),
				"error": err.Error(),
			}).Debug("This is not a valid URL")
			continue
		}

		err = utils.ValidateURL(URL)
		if err != nil {
			log.WithFields(log.Fields{
				"url":   scanner.Text(),
				"error": err.Error(),
			}).Debug("This is not a valid URL")
			continue
		}

		item := NewItem(URL, nil, 0)
		seeds = append(seeds, *item)
	}

	if err := scanner.Err(); err != nil {
		return seeds, err
	}

	if len(seeds) == 0 {
		return seeds, errors.New("Seed list's content invalid")
	}

	return seeds, nil
}
