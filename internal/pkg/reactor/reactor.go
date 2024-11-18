// Package reactor provides functionality to manage and control the processing of seeds.
package reactor

import (
	"context"
	"fmt"
	"sync"

	"github.com/internetarchive/Zeno/pkg/models"
)

// reactor struct holds the state and channels for managing seeds processing.
type reactor struct {
	tokenPool  chan struct{}   // Token pool to control asset count
	ctx        context.Context // Context for stopping the reactor
	cancelFunc context.CancelFunc
	input      chan *models.Seed // Combined input channel for source and feedback
	output     chan *models.Seed // Output channel
	stateTable sync.Map          // State table for tracking seeds by UUID
	wg         sync.WaitGroup    // WaitGroup to manage goroutines
}

var (
	globalReactor *reactor
	once          sync.Once
)

// Start initializes the global reactor with the given maximum tokens.
// This method can only be called once.
func Start(maxTokens int, outputChan chan *models.Seed) error {
	var done bool

	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		globalReactor = &reactor{
			tokenPool:  make(chan struct{}, maxTokens),
			ctx:        ctx,
			cancelFunc: cancel,
			input:      make(chan *models.Seed, maxTokens),
			output:     outputChan,
		}
		globalReactor.wg.Add(1)
		go globalReactor.run()
		fmt.Println("Reactor started")
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
		globalReactor.cancelFunc()
		globalReactor.wg.Wait()
		close(globalReactor.output)
		fmt.Println("Reactor stopped")
	}
}

// ReceiveFeedback sends an item to the feedback channel.
func ReceiveFeedback(item *models.Seed) error {
	if globalReactor == nil {
		return ErrReactorNotInitialized
	}

	item.Source = models.SeedSourceFeedback
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

// ReceiveSource sends an item to the source seeds channel.
func ReceiveSource(item *models.Seed) error {
	if globalReactor == nil {
		return ErrReactorNotInitialized
	}

	select {
	case globalReactor.tokenPool <- struct{}{}:
		globalReactor.stateTable.Store(item.UUID.String(), item)
		globalReactor.input <- item
		return nil
	case <-globalReactor.ctx.Done():
		return ErrReactorShuttingDown
	}
}

// MarkAsFinished marks an item as finished and releases a token if found in the state table.
func MarkAsFinished(item *models.Seed) error {
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
			fmt.Println("Reactor shutting down...")
			return

		// Feeds items to the output channel
		case item, ok := <-r.input:
			if ok {
				r.output <- item
			}
		}
	}
}
