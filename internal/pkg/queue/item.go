package queue

import (
	"fmt"
	"hash/fnv"
	"io"
	"net/url"

	"github.com/internetarchive/Zeno/internal/pkg/utils"
)

func NewItem(URL *url.URL, parentURL *url.URL, itemType string, hop uint64, ID string, bypassSeencheck bool) (*Item, error) {
	h := fnv.New64a()
	h.Write([]byte(utils.URLToString(URL)))

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
