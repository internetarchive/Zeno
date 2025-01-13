package postprocessor

import (
	"context"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/internetarchive/Zeno/pkg/models"
)

type postprocessor struct {
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	inputCh  chan *models.Item
	outputCh chan *models.Item
}

var (
	globalPostprocessor *postprocessor
	once                sync.Once
	logger              *log.FieldedLogger
)

// This functions starts the preprocessor responsible for preparing
// the seeds sent by the reactor for captures
func Start(inputChan, outputChan chan *models.Item) error {
	var done bool

	log.Start()
	logger = log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor",
	})

	stats.Init()

	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		globalPostprocessor = &postprocessor{
			ctx:      ctx,
			cancel:   cancel,
			inputCh:  inputChan,
			outputCh: outputChan,
		}
		logger.Debug("initialized")
		globalPostprocessor.wg.Add(1)
		go run()
		logger.Info("started")
		done = true
	})

	if !done {
		return ErrPostprocessorAlreadyInitialized
	}

	return nil
}

func Stop() {
	if globalPostprocessor != nil {
		globalPostprocessor.cancel()
		globalPostprocessor.wg.Wait()
		logger.Info("stopped")
	}
}

func run() {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.run",
	})

	defer globalPostprocessor.wg.Done()

	// Create a context to manage goroutines
	ctx, cancel := context.WithCancel(globalPostprocessor.ctx)
	defer cancel()

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Guard to limit the number of concurrent archiver routines
	guard := make(chan struct{}, config.Get().WorkersCount)

	// Subscribe to the pause controler
	controlChans := pause.Subscribe()
	defer pause.Unsubscribe(controlChans)

	for {
		select {
		case <-controlChans.PauseCh:
			logger.Debug("received pause event")
			controlChans.ResumeCh <- struct{}{}
			logger.Debug("received resume event")
		case item, ok := <-globalPostprocessor.inputCh:
			if ok {
				logger.Debug("received item", "item", item.GetShortID())
				guard <- struct{}{}
				wg.Add(1)
				stats.PostprocessorRoutinesIncr()
				go func(ctx context.Context) {
					defer wg.Done()
					defer func() { <-guard }()
					defer stats.PostprocessorRoutinesDecr()

					if item.GetStatus() == models.ItemFailed || item.GetStatus() == models.ItemCompleted {
						logger.Debug("skipping item", "item", item.GetShortID(), "status", item.GetStatus().String())
					} else {
						outlinks := postprocess(item)
						for _, outlink := range outlinks {
							logger.Info("sending outlink", "item", outlink.GetShortID())
							globalPostprocessor.outputCh <- outlink
						}
					}

					select {
					case globalPostprocessor.outputCh <- item:
					case <-ctx.Done():
						logger.Debug("aborting item due to stop", "item", item.GetShortID())
						return
					}
				}(ctx)
			}
		case <-globalPostprocessor.ctx.Done():
			logger.Debug("shutting down")
			wg.Wait()
			return
		}
	}
}

func postprocess(seed *models.Item) (outlinks []*models.Item) {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.postprocess",
	})

	// If we don't capture assets, there is no need to postprocess the item
	// TODO: handle hops even with disable assets capture
	if config.Get().DisableAssetsCapture {
		return
	}

	items, err := seed.GetNodesAtLevel(seed.GetMaxDepth())
	if err != nil {
		logger.Error("unable to get nodes at level", "err", err.Error(), "seed_id", seed.GetShortID())
		panic(err)
	}

	for i := range items {
		outlinks = postprocessItem(items[i], seed)

		// Once the item is postprocessed, we can close the body buffer if it exists.
		// It will release the resources and delete the temporary file (if any).
		if items[i].GetURL().GetBody() != nil {
			err = items[i].GetURL().GetBody().Close()
			if err != nil {
				logger.Error("unable to close body", "err", err.Error(), "item_id", items[i].GetShortID())
				panic(err)
			}

			items[i].GetURL().SetBody(nil)
		}
	}

	return
}
