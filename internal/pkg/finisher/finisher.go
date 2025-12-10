package finisher

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/config"
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
		for i := 0; i < config.Get().WorkersCount; i++ {
			stats.FinisherRoutinesIncr()
			globalFinisher.wg.Add(1)
			go globalFinisher.worker(strconv.Itoa(i))
		}
		logger.Info("started")
	})

	if globalFinisher == nil {
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

func (f *finisher) worker(workerID string) {
	defer func() {
		stats.FinisherRoutinesDecr()
		f.wg.Done()
	}()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "finisher.worker",
		"worker_id": workerID,
	})

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
		case seed, ok := <-f.inputCh:
			if ok {
				if err := f.handleSeed(seed, workerID, logger); err != nil {
					panic(err)
				}
			}
		}
	}
}

func (f *finisher) handleSeed(seed *models.Item, workerID string, logger *log.FieldedLogger) error {
	if seed == nil {
		return errors.New("received nil seed")
	}

	if !seed.IsSeed() {
		return errors.New("received non-seed item")
	}

	logger.Debug("received seed", "seed", seed.GetShortID())

	if err := seed.CheckConsistency(); err != nil {
		return fmt.Errorf("seed consistency check failed with err: for %s: %s", err.Error(), seed.GetShortID())
	}

	// If the seed is fresh, send it to the source
	if seed.GetStatus() == models.ItemFresh {
		logger.Debug("fresh seed received", "seed", seed)
		f.sourceProducedCh <- seed
		return nil
	}

	// If the seed has fresh children, send it to feedback
	if !seed.CompleteAndCheck() {
		logger.Debug("seed has fresh children", "seed", seed.GetShortID())
		if err := reactor.ReceiveFeedback(seed); err != nil && err != reactor.ErrReactorFrozen {
			return fmt.Errorf("worker %s: feedback failed for %s: %w", workerID, seed.GetShortID(), err)
		}
		return nil
	}

	// Otherwise mark as finished
	logger.Debug("seed has no fresh redirection or children", "seed", seed.GetShortID())
	if err := reactor.MarkAsFinished(seed); err != nil {
		return fmt.Errorf("worker %s: mark as finished failed for %s: %w", workerID, seed.GetShortID(), err)
	}

	// Notify the source that the seed has been finished
	// E.g.: to delete the seed in Crawl HQ
	if f.sourceFinishedCh != nil {
		f.sourceFinishedCh <- seed
	}

	stats.SeedsFinishedIncr()
	logger.Debug("seed finished", "seed", seed.GetShortID())

	return nil
}
