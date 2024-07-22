package queue

import (
	"fmt"
	"io"
)

func (q *PersistentGroupedQueue) loadMetadata() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	_, err := q.metadataFile.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek to start of metadata file: %w", err)
	}

	var metadata struct {
		Stats       QueueStats
		CurrentHost int
	}

	err = q.metadataDecoder.Decode(&metadata)
	if err != nil {
		if err == io.EOF {
			// No metadata yet, this might be a new queue
			return nil
		}
		return fmt.Errorf("failed to decode metadata: %w", err)
	}

	q.currentHost = metadata.CurrentHost
	q.stats = metadata.Stats

	// Reinitialize maps if they're nil
	if q.stats.ElementsPerHost == nil {
		q.stats.ElementsPerHost = make(map[string]int)
	}
	if q.stats.HostDistribution == nil {
		q.stats.HostDistribution = make(map[string]float64)
	}

	return nil
}

func (q *PersistentGroupedQueue) saveMetadata() error {
	q.statsMutex.RLock()
	defer q.statsMutex.RUnlock()

	_, err := q.metadataFile.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek to start of metadata file: %w", err)
	}

	err = q.metadataFile.Truncate(0)
	if err != nil {
		return fmt.Errorf("failed to truncate metadata file: %w", err)
	}

	metadata := struct {
		Stats       QueueStats
		CurrentHost int
	}{
		Stats:       q.stats,
		CurrentHost: q.currentHost,
	}

	err = q.metadataEncoder.Encode(metadata)
	if err != nil {
		return fmt.Errorf("failed to encode metadata: %w", err)
	}

	return nil
}
