package finisher

import (
	"context"
	"fmt"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/internetarchive/Zeno/pkg/models"
)

type finisher struct {
	ctx              context.Context
	cancel           context.CancelFunc
	inputCh          chan *models.Item
	sourceFinishedCh chan *models.Item
	sourceProducedCh chan *models.Item
	wg               sync.WaitGroup
}

var (
	globalFinisher *finisher
	once           sync.Once
	logger         *log.FieldedLogger
)

// Start initializes the global finisher with the given input channel.
// This method can only be called once.
func Start(inputChan, sourceFinishedChan, sourceProducedChan chan *models.Item) error {
	var done bool

	log.Start()
	logger = log.NewFieldedLogger(&log.Fields{
		"component": "finisher",
	})

	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		globalFinisher = &finisher{
			ctx:              ctx,
			cancel:           cancel,
			inputCh:          inputChan,
			sourceFinishedCh: sourceFinishedChan,
			sourceProducedCh: sourceProducedChan,
			wg:               sync.WaitGroup{},
		}
		logger.Debug("initialized")
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
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "finisher.run",
	})

	f.wg.Add(1)
	defer f.wg.Done()

	controlChans := pause.Subscribe()
	defer pause.Unsubscribe(controlChans)

	for {
		select {
		case <-f.ctx.Done():
			logger.Debug("shutting down")
			return
		case <-controlChans.PauseCh:
			logger.Debug("received pause event")
			controlChans.ResumeCh <- struct{}{}
			logger.Debug("received resume event")
		case item := <-f.inputCh:
			if item == nil {
				panic("received nil item")
			}

			if !item.IsSeed() {
				panic("received non-seed item")
			}

			logger.Debug("received item", "item", item.GetShortID())

			if err := item.CheckConsistency(); err != nil {
				panic(fmt.Sprintf("item consistency check failed with err: %s, item id %s", err.Error(), item.GetShortID()))
			}

			// If the item is fresh, send it to the source
			if item.GetStatus() == models.ItemFresh {
				logger.Debug("fresh item received", "item", item)
				f.sourceProducedCh <- item
				continue
			}

			// If the item has fresh children, send it to feedback
			isComplete := item.CompleteAndCheck()
			if !isComplete {
				logger.Debug("item has fresh children", "item", item.GetShortID())
				err := reactor.ReceiveFeedback(item)
				if err != nil && err != reactor.ErrReactorFrozen {
					panic(err)
				}
				continue
			}

			// If the item has no fresh redirection or children, mark it as finished
			logger.Debug("item has no fresh redirection or children", "item", item.GetShortID())
			err := reactor.MarkAsFinished(item)
			if err != nil {
				panic(err)
			}

			// Notify the source that the item has been finished
			// E.g.: to delete the item in Crawl HQ
			if f.sourceFinishedCh != nil {
				f.sourceFinishedCh <- item
			}

			stats.SeedsFinishedIncr()
			logger.Debug("item finished", "item", item.GetShortID())
		}
	}
}
