package index

import (
	"encoding/gob"
	"fmt"
	"io"
)

func (im *IndexManager) writeToWAL(Op Operation, host, id string, position, size uint64) error {
	// Write to WAL
	entry := WALEntry{Op: Op, Host: host, BlobID: id, Position: position, Size: size}
	if err := im.walEncoder.Encode(entry); err != nil {
		return fmt.Errorf("failed to write to WAL: %w", err)
	}
	if err := im.walFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync WAL: %w", err)
	}

	return nil
}

func (im *IndexManager) replayWAL(entriesReplayed *int) error {
	_, err := im.walFile.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek to the beginning of WAL: %w", err)
	}

	decoder := gob.NewDecoder(im.walFile)

	var tempEntriesReplayed int
	for {
		var entry WALEntry
		if err := decoder.Decode(&entry); err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("failed to decode WAL entry: %w", err)
		}

		switch entry.Op {
		case OpAdd:
			err := im.hostIndex.add(entry.Host, entry.BlobID, entry.Position, entry.Size)
			if err != nil {
				return fmt.Errorf("failed to replay WAL entry: %w", err)
			}
		case OpPop:
			err := im.hostIndex.removeBlob(entry.Host, entry.BlobID)
			if err != nil {
				return fmt.Errorf("failed to replay WAL entry: %w", err)
			}
		}
		tempEntriesReplayed++
	}

	fmt.Printf("Replayed %d entries from WAL\n", tempEntriesReplayed)

	if tempEntriesReplayed == 0 {
		return ErrNoWALEntriesReplayed
	}

	if entriesReplayed != nil {
		*entriesReplayed = tempEntriesReplayed
	}
	return nil
}

func (im *IndexManager) truncateWAL() error {
	if err := im.walFile.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate WAL file: %w", err)
	}
	_, err := im.walFile.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek WAL file: %w", err)
	}
	return nil
}

func (im *IndexManager) isWALEmpty() (bool, error) {
	stat, err := im.walFile.Stat()
	if err != nil {
		return false, fmt.Errorf("failed to get WAL file info: %w", err)
	}
	return stat.Size() == 0, nil
}
