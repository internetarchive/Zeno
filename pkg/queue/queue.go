package queue

import (
	"path/filepath"

	"github.com/beeker1121/goque"
	"github.com/google/uuid"
)

// NewQueue initialize a new goque.PrefixQueue
func NewQueue() (queue *goque.PrefixQueue, err error) {
	// All on-disk queues are in the "./jobs" directory
	queueUUID, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}
	queuePath := filepath.Join(".", "jobs", queueUUID.String())

	// Initialize a prefix queue
	queue, err = goque.OpenPrefixQueue(queuePath)
	if err != nil {
		return nil, err
	}

	return queue, nil
}
