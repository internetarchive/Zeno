package queue

import (
	"bufio"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/url"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/gosuri/uilive"
	"github.com/internetarchive/Zeno/internal/utils"
	"github.com/sirupsen/logrus"
)

func NewItem(URL *url.URL, parentURL *url.URL, itemType string, hop uint64, ID string, bypassSeencheck bool) (*Item, error) {
	h := fnv.New64a()
	h.Write([]byte(utils.URLToString(URL)))

	if ID == "" {
		ID = uuid.New().String()
	}

	return &Item{
		URL:             URL,
		ParentURL:       parentURL,
		Hop:             hop,
		Type:            itemType,
		ID:              ID,
		Hash:            h.Sum64(),
		BypassSeencheck: bypassSeencheck,
	}, nil
}

func (q *PersistentGroupedQueue) ReadItemAt(position uint64, itemSize uint64) ([]byte, error) {
	// Ensure the file pointer is at the correct position
	_, err := q.queueFile.Seek(int64(position), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to item position: %w", err)
	}

	// Read the specific number of bytes for the item
	itemBytes := make([]byte, itemSize)
	_, err = io.ReadFull(q.queueFile, itemBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read item bytes: %w", err)
	}

	return itemBytes, nil
}

func FileToItems(path string) (seeds []Item, err error) {
	var totalCount, validCount int

	writer := uilive.New()
	// We manual flushing, uilive's auto flushing is not needed
	// set it to 1s for convenience
	writer.RefreshInterval = 1 * time.Second
	writerFlushInterval := 50 * time.Millisecond
	writerFlushed := time.Now()
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

		item, err := NewItem(URL, nil, "seed", 0, "", false)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"url": scanner.Text(),
				"err": err.Error(),
			}).Debug("Failed to create new item")
			continue
		}

		seeds = append(seeds, *item)
		validCount++

		if time.Since(writerFlushed) > writerFlushInterval {
			fmt.Fprintf(writer, "\t   Reading input list.. Found %d valid URLs out of %d URLs read...\n", validCount, totalCount)
			writer.Flush()
			writerFlushed = time.Now()
		}
	}
	writer.Stop()

	if err := scanner.Err(); err != nil {
		return seeds, err
	}

	if len(seeds) == 0 {
		return seeds, errors.New("seed list's content invalid")
	}

	logrus.WithFields(logrus.Fields{
		"total": totalCount,
		"valid": validCount,
	}).Info("Finished reading input list")

	return seeds, nil
}
