package preprocessor

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor/seencheck"
	"github.com/internetarchive/Zeno/internal/pkg/source/hq"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/internetarchive/Zeno/pkg/models"
)

type preprocessor struct {
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	inputCh  chan *models.Item
	outputCh chan *models.Item
}

var (
	globalPreprocessor *preprocessor
	once               sync.Once
	logger             *log.FieldedLogger
)

// This functions starts the preprocessor responsible for preparing
// the seeds sent by the reactor for captures
func Start(inputChan, outputChan chan *models.Item) error {
	var done bool

	log.Start()
	logger = log.NewFieldedLogger(&log.Fields{
		"component": "preprocessor",
	})

	stats.Init()

	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		globalPreprocessor = &preprocessor{
			ctx:      ctx,
			cancel:   cancel,
			inputCh:  inputChan,
			outputCh: outputChan,
		}
		logger.Debug("initialized")
		globalPreprocessor.wg.Add(1)
		go run()
		logger.Info("started")
		done = true
	})

	if !done {
		return ErrPreprocessorAlreadyInitialized
	}

	return nil
}

func Stop() {
	if globalPreprocessor != nil {
		globalPreprocessor.cancel()
		globalPreprocessor.wg.Wait()
		logger.Info("stopped")
	}
}

func run() {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "preprocessor.run",
	})

	defer globalPreprocessor.wg.Done()

	// Create a context to manage goroutines
	ctx, cancel := context.WithCancel(globalPreprocessor.ctx)
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
		// Closes the run routine when context is canceled
		case <-globalPreprocessor.ctx.Done():
			logger.Debug("shutting down")
			return
		case item, ok := <-globalPreprocessor.inputCh:
			if ok {
				logger.Debug("received item", "item", item.GetShortID())
				guard <- struct{}{}
				wg.Add(1)
				stats.PreprocessorRoutinesIncr()
				go func(ctx context.Context) {
					defer wg.Done()
					defer func() { <-guard }()
					defer stats.PreprocessorRoutinesDecr()

					preprocess(item)

					select {
					case <-ctx.Done():
						return
					case globalPreprocessor.outputCh <- item:
					}
				}(ctx)
			}
		}
	}
}

func preprocess(item *models.Item) {
	// Validate the URL of either the item itself and/or its childs
	// TODO: if an error happen and it's a fresh item, we should mark it as failed in HQ (if it's a HQ-based crawl)
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "preprocessor.process",
	})

	items, err := item.GetNodesAtLevel(item.GetMaxDepth())
	if err != nil {
		panic(err)
	}

	for i := range items {
		// Discard any child that is not fresh
		if items[i].GetStatus() != models.ItemFresh {
			continue
		}

		// Normalize the URL
		if !items[i].IsSeed() {
			err := normalizeURL(items[i].GetURL(), items[i].GetParent().GetURL())
			if err != nil {
				logger.Debug("unable to validate URL", "url", items[i].GetURL().Raw, "err", err.Error())
				items[i].GetParent().RemoveChild(items[i])
				continue
			}
		}
		// TODO : normalize seeds
		//
		// else {
		// 	err := normalizeURL(items[i].GetURL(), &models.URL{Raw: items[i].GetSeedVia()})
		// 	if err != nil {
		// 		logger.Debug("unable to validate URL", "url", items[i].GetURL().Raw, "err", err.Error())
		// 		items[i].SetError(models.ErrFailedAtPreprocessor)
		// 		continue
		// 	}
		// }

		// If we are processing assets, then we need to remove childs that are just domains
		// (which means that they are not assets, but false positives)
		if items[i].IsChild() {
			if items[i].GetURL().GetParsed().Path == "" || items[i].GetURL().GetParsed().Path == "/" {
				logger.Debug("removing child with empty path", "url", items[i].GetURL().Raw)
				items[i].GetParent().RemoveChild(items[i])
			}
		}
	}

	// Deduplicate items based on their URL and remove duplicates
	item.DedupeItems()

	// If the item is a redirection or an asset, we need to seencheck it if needed
	if config.Get().UseHQ {
		err = hq.SeencheckItem(item)
		if err != nil {
			logger.Warn("unable to seencheck item", "id", item.GetShortID(), "err", err.Error(), "func", "preprocessor.preprocess")
		}
	} else {
		err = seencheck.SeencheckItem(item)
		if err != nil {
			logger.Warn("unable to seencheck item", "id", item.GetShortID(), "err", err.Error(), "func", "preprocessor.preprocess")
		}
	}

	// Recreate the items list after deduplication and seencheck
	items, err = item.GetNodesAtLevel(item.GetMaxDepth())
	if err != nil {
		panic(err)
	}

	// Remove any item that is not fresh from the list
	for i := range items {
		if items[i].GetStatus() != models.ItemFresh {
			items = append(items[:i], items[i+1:]...)
		}
	}

	if len(items) == 0 {
		logger.Warn("no more work to do", "item", item.GetShortID())
		item.SetStatus(models.ItemCompleted)
		return
	}

	// Finally, we build the requests, applying any site-specific behavior needed
	for i := range items {
		// TODO: apply site-specific stuff
		req, err := http.NewRequest(http.MethodGet, items[i].GetURL().String(), nil)
		if err != nil {
			panic(fmt.Sprintf("unable to create request for URL %s: %s", items[i].GetURL().String(), err.Error()))
		}

		items[i].GetURL().SetRequest(req)
		items[i].SetStatus(models.ItemPreProcessed)
	}
}
