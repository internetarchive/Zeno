package finisher

import (
	"context"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
	"github.com/internetarchive/Zeno/pkg/models"
)

type finisher struct {
	ctx     context.Context
	cancel  context.CancelFunc
	inputCh chan *models.Item
	errorCh chan *models.Item
	wg      sync.WaitGroup
}

var (
	globalFinisher *finisher
	once           sync.Once
	logger         *log.FieldedLogger
)

// Start initializes the global finisher with the given input channel.
// This method can only be called once.
func Start(inputChan, errorChan chan *models.Item) error {
	var done bool

	log.Start()
	logger = log.NewFieldedLogger(&log.Fields{
		"component": "finisher",
	})

	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		globalFinisher = &finisher{
			ctx:     ctx,
			cancel:  cancel,
			inputCh: inputChan,
			errorCh: errorChan,
		}
		logger.Debug("initialized")
		globalFinisher.wg.Add(1)
		go globalFinisher.run()
		logger.Info("started")
		done = true
	})

	if !done {
		return ErrFinisherAlreadyInitialized
	}

	return nil
}

// Stop stops the global finisher.
func Stop() {
	if globalFinisher != nil {
		logger.Debug("received stop signal")
		globalFinisher.cancel()
		globalFinisher.wg.Wait()
		globalFinisher = nil
		once = sync.Once{}
		logger.Info("stopped")
	}
}

func (f *finisher) run() {
	defer f.wg.Done()

	for {
		select {
		case <-f.ctx.Done():
			logger.Info("shutting down")
			return
		case item := <-f.inputCh:
			if item == nil {
				panic("received nil item")
			}

			logger.Debug("received item", "item", item.UUID.String())
			if item.Error != nil {
				logger.Error("received item with error", "item", item.UUID.String(), "error", item.Error)
				f.errorCh <- item
				continue
			}

			reactor.MarkAsFinished(item)

			logger.Info("item finished", "item", item.UUID.String())
		case item := <-f.errorCh:
			if item == nil {
				panic("received nil item")
			}

			logger.Info("received item with error", "item", item.UUID.String(), "error", item.Error)

			reactor.MarkAsFinished(item)

			logger.Info("item with error finished", "item", item.UUID.String())
		}
	}
}
