package queue

import "errors"

var (
	// ErrQueueFull is returned when the queue is full
	ErrQueueFull = errors.New("queue is full")

	// ErrQueueEmpty is returned when the queue is empty
	ErrQueueEmpty = errors.New("queue is empty")

	// ErrQueueClosed is returned when the queue is closed
	ErrQueueClosed = errors.New("queue is closed")

	// ErrQueueTimeout is returned when the queue operation times out
	ErrQueueTimeout = errors.New("queue operation timed out")

	// ErrQueueAlreadyClosed is returned when the queue is already closed
	ErrQueueAlreadyClosed = errors.New("queue is already closed")

	// ErrNoHostsInQueue is returned when there are no hosts in the queue
	ErrNoHostsInQueue = errors.New("no hosts in queue")
)
