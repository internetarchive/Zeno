// Package reactor provides functionality to manage and control the processing of seeds.
package reactor

import (
	"context"
	"log/slog"
	"sync"

	"github.com/internetarchive/Zeno/pkg/models"
)

// reactor struct holds the state and channels for managing seeds processing.
type reactor struct {
	tokenPool  chan struct{}      // Token pool to control asset count
	ctx        context.Context    // Context for stopping the reactor
	cancel     context.CancelFunc // Context's cancel func
	input      chan *models.Item  // Combined input channel for source and feedback
	output     chan *models.Item  // Output channel
	stateTable sync.Map           // State table for tracking seeds by UUID
	wg         sync.WaitGroup     // WaitGroup to manage goroutines
}

var (
	globalReactor *reactor
	once          sync.Once
)

// Start initializes the global reactor with the given maximum tokens.
// This method can only be called once.
func Start(maxTokens int, outputChan chan *models.Item) error {
	var done bool

	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		globalReactor = &reactor{
			tokenPool: make(chan struct{}, maxTokens),
			ctx:       ctx,
			cancel:    cancel,
			input:     make(chan *models.Item, maxTokens),
			output:    outputChan,
		}
		globalReactor.wg.Add(1)
		go globalReactor.run()
		slog.Info("reactor started")
		done = true
	})

	if !done {
		return ErrReactorAlreadyInitialized
	}

	return nil
}

// Stop stops the global reactor and waits for all goroutines to finish.
func Stop() {
	if globalReactor != nil {
		globalReactor.cancel()
		globalReactor.wg.Wait()
		close(globalReactor.output)
		slog.Info("reactor stopped")
	}
}

// ReceiveFeedback sends an item to the feedback channel.
// If the item is not present on the state table it gets discarded
func ReceiveFeedback(item *models.Item) error {
	if globalReactor == nil {
		return ErrReactorNotInitialized
	}

	item.Source = models.ItemSourceFeedback
	_, loaded := globalReactor.stateTable.Swap(item.UUID.String(), item)
	if !loaded {
		// An item sent to the feedback channel should be present on the state table, if not present reactor should error out
		return ErrFeedbackItemNotPresent
	}
	select {
	case globalReactor.input <- item:
		return nil
	case <-globalReactor.ctx.Done():
		return ErrReactorShuttingDown
	}
}

// ReceiveInsert sends an item to the input channel consuming a token.
// It is the responsibility of the sender to set either ItemSourceQueue or ItemSourceHQ, if not set seed will get forced ItemSourceInsert
func ReceiveInsert(item *models.Item) error {
	if globalReactor == nil {
		return ErrReactorNotInitialized
	}

	select {
	case globalReactor.tokenPool <- struct{}{}:
		if item.Source != models.ItemSourceQueue && item.Source != models.ItemSourceHQ {
			item.Source = models.ItemSourceInsert
		}
		globalReactor.stateTable.Store(item.UUID.String(), item)
		globalReactor.input <- item
		return nil
	case <-globalReactor.ctx.Done():
		return ErrReactorShuttingDown
	}
}

// MarkAsFinished marks an item as finished and releases a token if found in the state table.
func MarkAsFinished(item *models.Item) error {
	if globalReactor == nil {
		return ErrReactorNotInitialized
	}

	if _, loaded := globalReactor.stateTable.LoadAndDelete(item.UUID.String()); loaded {
		<-globalReactor.tokenPool
		return nil
	}
	return ErrFinisehdItemNotFound
}

func (r *reactor) run() {
	defer r.wg.Done()

	for {
		select {
		// Closes the run routine when context is canceled
		case <-r.ctx.Done():
			slog.Info("reactor shutting down")
			return

		// Feeds items to the output channel
		case item, ok := <-r.input:
			if ok {
				r.output <- item
			}
		}
	}
}
