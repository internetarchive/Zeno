package index

import (
	"encoding/gob"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

var dumpFrequency = 60 // seconds

type Operation int

const (
	OpAdd Operation = iota
	OpPop
)

type WALEntry struct {
	Op       Operation
	Host     string
	BlobID   string
	Position uint64
	Size     uint64
}

type IndexManager struct {
	sync.Mutex
	hostIndex    *Index
	indexFile    *os.File
	queueDirPath string
	indexEncoder *gob.Encoder
	indexDecoder *gob.Decoder
	dumpTicker   *time.Ticker
	lastDumpTime time.Time
	opsSinceDump int
	totalOps     uint64

	// WAL related fields

	walFile     *os.File
	walEncoder  *gob.Encoder
	walDecoder  *gob.Decoder
	walCommit   *atomic.Uint64 // Flying in memory commit id
	walCommited *atomic.Uint64 // Synced to disk commit id
	// Number of listeners waiting for walCommitedNotify.
	// It must be accurate, otherwise walNotifyListeners will be blocked
	walNotifyListeners *atomic.Int64
	walCommitedNotify  chan uint64 // receives the commited id from walCommitsSyncer
	walSyncerRunning   atomic.Bool // used to prevent multiple walCommitsSyncer running,
	walStopChan        chan struct{}
	WAL_IO_PERCENT     int           // [1, 100] limit max io percentage for WAL sync
	WAL_MIN_INTERVAL   time.Duration // minimum interval **between** between after-sync and next sync

	stopChan chan struct{}
}

// NewIndexManager creates a new IndexManager instance and loads the index from the index file.
func NewIndexManager(walPath, indexPath, queueDirPath string) (*IndexManager, error) {
	walFile, err := os.OpenFile(walPath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	indexFile, err := os.OpenFile(indexPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		walFile.Close()
		return nil, fmt.Errorf("failed to open index file: %w", err)
	}

	im := &IndexManager{
		hostIndex:          newIndex(),
		walFile:            walFile,
		indexFile:          indexFile,
		queueDirPath:       queueDirPath,
		walEncoder:         gob.NewEncoder(walFile),
		walDecoder:         gob.NewDecoder(walFile),
		indexEncoder:       gob.NewEncoder(indexFile),
		indexDecoder:       gob.NewDecoder(indexFile),
		dumpTicker:         time.NewTicker(time.Duration(dumpFrequency) * time.Second),
		lastDumpTime:       time.Now(),
		walCommit:          new(atomic.Uint64),
		walCommited:        new(atomic.Uint64),
		walNotifyListeners: new(atomic.Int64),
		walCommitedNotify:  make(chan uint64),
		WAL_IO_PERCENT:     10,
		WAL_MIN_INTERVAL:   10 * time.Millisecond,
		walStopChan:        make(chan struct{}),
		stopChan:           make(chan struct{}),
	}

	// Check if WAL file is empty
	im.Lock()
	empty, err := im.unsafeIsWALEmpty() // FIXME: check error
	im.Unlock()
	if !empty {
		err := im.RecoverFromCrash()
		if err != nil {
			walFile.Close()
			indexFile.Close()
			return nil, fmt.Errorf("failed to recover from crash: %w", err)
		}
		fmt.Println("Recovered from crash")
	} else {
		err = im.loadIndex()
		if err != nil {
			walFile.Close()
			indexFile.Close()
			return nil, fmt.Errorf("failed to load index: %w", err)
		}
	}

	// Start the periodic dump goroutine
	periodicDumpStopChan := make(chan struct{})
	periodicDumpErrChan := make(chan error)
	go func(im *IndexManager, errChan chan error, stopChan chan struct{}) {
		for {
			select {
			case stop := <-im.stopChan:
				periodicDumpStopChan <- stop
				return
			case err := <-errChan:
				if err != nil {
					slog.Error("Periodic dump failed", "error", err) // No better way to log this, will wait for https://github.com/internetarchive/Zeno/issues/92
				}
			}
		}
	}(im, periodicDumpErrChan, periodicDumpStopChan)

	go im.periodicDump(periodicDumpErrChan, periodicDumpStopChan)
	go im.walCommitsSyncer()

	return im, nil
}

func (im *IndexManager) unsafeWalSync() error {
	return im.walFile.Sync()
}

func (im *IndexManager) walCommitsSyncer() {
	if swaped := im.walSyncerRunning.CompareAndSwap(false, true); !swaped {
		panic("walCommitsSyncer already running")
	}
	defer im.walSyncerRunning.Store(false)

	lastSyncDuration := time.Duration(0)
	for {
		if im.WAL_IO_PERCENT < 1 || im.WAL_IO_PERCENT > 100 {
			panic(fmt.Errorf("invalid WAL_IO_PERCENT: %d", im.WAL_IO_PERCENT))
		}

		sleepTime := lastSyncDuration * time.Duration((100-im.WAL_IO_PERCENT)/im.WAL_IO_PERCENT)
		if sleepTime < im.WAL_MIN_INTERVAL {
			sleepTime = im.WAL_MIN_INTERVAL
		}
		// fmt.Println("lastSyncDuration", lastSyncDuration, "sleepTime", sleepTime)
		time.Sleep(sleepTime)

		start := time.Now()
		flyingCommit := im.walCommit.Load()
		im.Lock()
		if err := im.unsafeWalSync(); err != nil {
			slog.Error("failed to sync WAL", "error", err)
			im.Unlock()
			return // FIXME: handle this error
		}
		im.Unlock()
		commited := flyingCommit
		lastSyncDuration = time.Since(start)

		im.walCommited.Store(commited)

		// Clear notify channel before sending, just in case.
		// should never happen if listeners number is accurate.
		for len(im.walCommitedNotify) > 0 {
			<-im.walCommitedNotify
			slog.Warn("unconsumed commited id in walCommitedNotify")
		}

		// Send the commited id to all listeners
		listeners := im.walNotifyListeners.Load()
		for i := int64(0); i < listeners; i++ {
			im.walCommitedNotify <- commited
		}

		// Check if we should stop
		select {
		case <-im.walStopChan:
			return
		default:
		}
	}
}

func (im *IndexManager) IsWALCommited(commit uint64) bool {
	return im.walCommited.Load() >= commit
}

// increments the WAL commit counter and returns the new commit ID.
func (im *IndexManager) WALCommit() uint64 {
	return im.walCommit.Add(1)
}

// AwaitWALCommited blocks until the given commit ID is commited.
func (im *IndexManager) AwaitWALCommited(commit uint64) {
	if commit == 0 {
		slog.Warn("AwaitWALCommited called with commit 0")
		return
	}
	if !im.walSyncerRunning.Load() {
		slog.Warn("AwaitWALCommited called without Syncer running, beaware of hanging")
	}
	if im.IsWALCommited(commit) {
		return
	}

	for {
		im.walNotifyListeners.Add(1)
		idFromChan := <-im.walCommitedNotify
		im.walNotifyListeners.Add(-1)

		if idFromChan >= commit {
			return
		}
	}
}

func (im *IndexManager) Add(host string, id string, position uint64, size uint64) (commit uint64, err error) {
	im.Lock()
	defer im.Unlock()

	// Write to WAL
	err = im.unsafeWriteToWAL(OpAdd, host, id, position, size)
	if err != nil {
		return 0, fmt.Errorf("failed to write to WAL: %w", err)
	}
	commit = im.WALCommit()

	// Update in-memory index
	if err := im.hostIndex.add(host, id, position, size); err != nil {
		return commit, fmt.Errorf("failed to update in-memory index: %w", err)
	}

	im.opsSinceDump++
	im.totalOps++

	return commit, nil
}

// Pop removes the oldest blob from the specified host's queue and returns its ID, position, and size.
// Pop is responsible for synchronizing the pop of the blob from the in-memory index and writing to the WAL.
// First it starts a goroutine that waits for the to-be-popped blob infos through blobChan, then writes to the WAL and if successful
// informs index.pop() through WALSuccessChan to either continue as normal or return an error.
func (im *IndexManager) Pop(host string) (commit uint64, id string, position uint64, size uint64, err error) {
	im.Lock()
	defer im.Unlock()
	// Prepare the channels
	blobChan := make(chan *blob)
	WALSuccessChan := make(chan bool)
	errChan := make(chan error)
	defer close(blobChan)
	defer close(WALSuccessChan)
	defer close(errChan)

	go func() {
		// Write to WAL
		blob := <-blobChan
		// If the blob is nil, it means index.pop() returned an error
		if blob == nil {
			return
		}
		err := im.unsafeWriteToWAL(OpPop, host, blob.id, blob.position, blob.size)
		if err != nil {
			errChan <- fmt.Errorf("failed to write to WAL: %w", err)
			WALSuccessChan <- false
		}
		id = blob.id
		position = blob.position
		size = blob.size
		WALSuccessChan <- true
		errChan <- nil
	}()

	// Pop from in-memory index
	err = im.hostIndex.pop(host, blobChan, WALSuccessChan)
	if err != nil {
		return 0, "", 0, 0, err
	}

	if err := <-errChan; err != nil {
		return 0, "", 0, 0, err
	}

	commit = im.WALCommit()

	im.opsSinceDump++
	im.totalOps++

	return
}

func (im *IndexManager) Close() error {
	im.dumpTicker.Stop()
	im.stopChan <- struct{}{}
	im.walStopChan <- struct{}{}
	if im.walSyncerRunning.Load() {
		panic("walCommitsSyncer still running")
	}
	if err := im.performDump(); err != nil {
		return fmt.Errorf("failed to perform final dump: %w", err)
	}
	if err := im.walFile.Close(); err != nil {
		return fmt.Errorf("failed to close WAL file: %w", err)
	}
	if err := im.indexFile.Close(); err != nil {
		return fmt.Errorf("failed to close index file: %w", err)
	}
	return nil
}

func (im *IndexManager) GetStats() string {
	im.Lock()
	defer im.Unlock()

	return fmt.Sprintf("Total operations: %d, Operations since last dump: %d",
		im.totalOps, im.opsSinceDump)
}

// GetHosts returns a list of all hosts in the index
func (im *IndexManager) GetHosts() []string {
	im.Lock()
	defer im.Unlock()

	return im.hostIndex.getOrderedHosts()
}

func (im *IndexManager) IsEmpty() bool {
	im.Lock()
	defer im.Unlock()

	im.hostIndex.Lock()
	defer im.hostIndex.Unlock()

	return len(im.hostIndex.index) == 0
}
