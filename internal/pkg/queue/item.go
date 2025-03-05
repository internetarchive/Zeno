package queue

import (
	"bufio"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
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

func FetchRemoteList(remoteURL string) (seeds []Item, err error) {
	var totalCount, validCount int

	// Validate the URL
	parsedURL, err := url.ParseRequestURI(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Perform HTTP GET request
	resp, err := http.Get(parsedURL.String())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status code: %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") && 
	   !strings.Contains(contentType, "text/csv") && 
	   !strings.Contains(contentType, "application/text") {
		slog.Warn("Unexpected content type", "type", contentType)
	}

	// Create a scanner to read the response body
	scanner := bufio.NewScanner(resp.Body)
	
	slog.Info("Start reading remote input list", "url", remoteURL)

	for scanner.Scan() {
		totalCount++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		URL, err := url.Parse(line)
		if err != nil {
			slog.Warn("Invalid URL", "url", line, "error", err.Error())
			continue
		}

		item, err := NewItem(URL, nil, "seed", 0, "", false)
		if err != nil {
			slog.Warn("Failed to create new item", "url", line, "error", err.Error())
			continue
		}

		seeds = append(seeds, *item)
		validCount++
	}

	if err := scanner.Err(); err != nil {
		return seeds, fmt.Errorf("error reading remote list: %w", err)
	}

	if len(seeds) == 0 {
		return seeds, errors.New("seed list is empty")
	}

	slog.Info("Finished reading remote input list", 
		"total", totalCount, 
		"valid", validCount, 
		"url", remoteURL)

	return seeds, nil
}

func FileToItems(path string) (seeds []Item, err error) {
	var totalCount, validCount int

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

	slog.Info("Start reading input list", "path", path)

	for scanner.Scan() {
		totalCount++
		URL, err := url.Parse(scanner.Text())
		if err != nil {
			slog.Warn("Invalid URL", "url", scanner.Text(), "error", err.Error())
			continue
		}

		item, err := NewItem(URL, nil, "seed", 0, "", false)
		if err != nil {
			slog.Warn("Failed to create new item", "url", scanner.Text(), "error", err.Error())
			continue
		}

		seeds = append(seeds, *item)
		validCount++
	}
	if err := scanner.Err(); err != nil {
		return seeds, err
	}

	if len(seeds) == 0 {
		return seeds, errors.New("seed list is empty")
	}

	slog.Info("Finished reading input list", "total", totalCount, "valid", validCount)

	return seeds, nil
}
