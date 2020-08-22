package queue

import (
	"path/filepath"

	"github.com/beeker1121/goque"
	"github.com/google/uuid"
)

// NewQueue initialize a new goque PriorityQueue
func NewQueue() (queue *goque.PriorityQueue, err error) {
	// All on-disk queues are in the "./jobs" directory
	queueUUID, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}
	queuePath := filepath.Join(".", "jobs", queueUUID.String())

	// Initialize a prefix queue
	queue, err = goque.OpenPriorityQueue(queuePath, goque.ASC)
	if err != nil {
		return nil, err
	}

	return queue, nil
}
