package get

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/gosuri/uilive"
	"github.com/internetarchive/Zeno/internal/pkg/queue"
	"github.com/sirupsen/logrus"
)

func isSeedList(path string) (seeds []queue.Item, err error) {
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
				"url": scanner.Text(),
				"err": err.Error(),
			}).Debug("this is not a valid URL")
			continue
		}

		item, err := queue.NewItem(URL, nil, "seed", 0, "", false)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"url": scanner.Text(),
				"err": err.Error(),
			}).Debug("Failed to create new item")
			continue
		}

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
		return seeds, errors.New("seed list's content invalid")
	}

	return seeds, nil
}
