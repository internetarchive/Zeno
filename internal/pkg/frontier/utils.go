package frontier

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/gosuri/uilive"
	"github.com/sirupsen/logrus"
)

// IsSeedList validates if the path is a seed list, and return an array of
// frontier.Item made of the seeds if it can
func IsSeedList(path string) (seeds []Item, err error) {
	var totalCount, validCount int
	writer := uilive.New()
	writer.Start()

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
	logrus.WithFields(logrus.Fields{
		"path": path,
	}).Info("Start reading input list")
	for scanner.Scan() {
		totalCount++
		URL, err := url.Parse(scanner.Text())
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"url":   scanner.Text(),
				"error": err.Error(),
			}).Debug("This is not a valid URL")
			continue
		}

		err = utils.ValidateURL(URL)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"url":   scanner.Text(),
				"error": err.Error(),
			}).Debug("This is not a valid URL")
			continue
		}

		item := NewItem(URL, nil, 0)
		seeds = append(seeds, *item)
		validCount++
		fmt.Fprintf(writer, "\t   Reading input list.. Found %d valid URLs out of %d URLs read.\n", validCount, totalCount)
		writer.Flush()
	}
	writer.Stop()

	if err := scanner.Err(); err != nil {
		return seeds, err
	}

	if len(seeds) == 0 {
		return seeds, errors.New("Seed list's content invalid")
	}

	return seeds, nil
}
