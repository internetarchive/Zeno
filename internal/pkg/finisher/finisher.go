package finisher

import (
	"context"
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
	controlChans := pause.Subscribe()
	defer pause.Unsubscribe(controlChans)
	defer f.wg.Done()

	for {
		select {
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

			// If the item is fresh, send it to the source
			if item.GetStatus() == models.ItemFresh {
				logger.Debug("fresh item received", "item", item)
				f.sourceProducedCh <- item
				continue
			}

			children, err := item.GetNodesAtLevel(item.GetMaxDepth())
			if err != nil {
				panic(err)
			}

			// If the item has fresh children, send it to feedback
			if len(children) > 0 {
				var doneFeedback bool
				for _, child := range children {
					if child.GetStatus() == models.ItemFresh {
						logger.Debug("item has fresh children", "item", item.GetShortID())
						err := reactor.ReceiveFeedback(item)
						if err != nil {
							panic(err)
						}
						doneFeedback = true
						break
					}
				}

				// If the item has fresh children, skip the rest of the select statement
				if doneFeedback {
					continue
				}
			}

			// If the item has no fresh redirection or children, mark it as finished
			logger.Debug("item has no fresh redirection or children", "item", item.GetShortID())
			err = reactor.MarkAsFinished(item)
			if err != nil {
				panic(err)
			}

			// Notify the source that the item has been finished
			// E.g.: to delete the item in Crawl HQ
			if f.sourceFinishedCh != nil {
				f.sourceFinishedCh <- item
			}

			stats.SeedsFinishedIncr()
			logger.Info("item finished", "item", item.GetShortID())
		case <-f.ctx.Done():
			logger.Debug("shutting down")
			return
		}
	}
}
