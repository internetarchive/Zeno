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
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor/sitespecific/tiktok"
	"github.com/internetarchive/Zeno/internal/pkg/source/hq"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
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

					if item.GetStatus() == models.ItemFailed || item.GetStatus() == models.ItemCompleted {
						panic(fmt.Sprintf("preprocessor received item with status %d, item id: %s", item.GetStatus(), item.GetShortID()))
					}

					preprocess(item)

					select {
					case globalPreprocessor.outputCh <- item:
					case <-ctx.Done():
						logger.Debug("aborting item due to stop", "item", item.GetShortID())
						return
					}
				}(ctx)
			}
		case <-globalPreprocessor.ctx.Done():
			logger.Debug("shutting down")
			wg.Wait()
			return
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
		// Panic on any child that is not fresh
		// This means that an incorrect item was inserted and/or that the finisher is not working correctly
		if items[i].GetStatus() != models.ItemFresh {
			panic(fmt.Sprintf("non-fresh item received in preprocessor: %s", items[i].GetStatus().String()))
		}

		// Normalize the URL
		if items[i].IsSeed() {
			err := normalizeURL(items[i].GetURL(), nil)
			if err != nil {
				logger.Debug("unable to validate URL", "url", items[i].GetURL().Raw, "err", err.Error())
				items[i].SetStatus(models.ItemCompleted)
				return
			}
		} else {
			err := normalizeURL(items[i].GetURL(), items[i].GetParent().GetURL())
			if err != nil {
				logger.Debug("unable to validate URL", "url", items[i].GetURL().Raw, "err", err.Error())
				items[i].GetParent().RemoveChild(items[i])
				continue
			}
		}

		// Verify if the URL isn't to be excluded
		if utils.StringContainsSliceElements(items[i].GetURL().GetParsed().Host, config.Get().ExcludeHosts) ||
			utils.StringContainsSliceElements(items[i].GetURL().GetParsed().Path, config.Get().ExcludeString) ||
			matchRegexExclusion(items[i]) {
			logger.Debug("URL excluded", "url", items[i].GetURL().String())
			if items[i].IsChild() {
				items[i].GetParent().RemoveChild(items[i])
			} else {
				items[i].SetStatus(models.ItemCompleted)
				return
			}
			continue
		}

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

	items, err = item.GetNodesAtLevel(item.GetMaxDepth())
	if err != nil {
		panic(err)
	}

	var foundFresh bool
	for i := range items {
		if items[i].GetStatus() == models.ItemFresh {
			foundFresh = true
			break
		}
	}

	if len(items) == 0 || !foundFresh {
		logger.Warn("no more work to do after dedupe", "item", item.GetShortID())
		item.SetStatus(models.ItemCompleted)
		return
	}

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
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].GetStatus() != models.ItemFresh {
			items = append(items[:i], items[i+1:]...)
		}
	}

	if len(items) == 0 {
		logger.Warn("no more work to do after seencheck", "item", item.GetShortID())
		item.SetStatus(models.ItemCompleted)
		return
	}

	// Finally, we build the requests, applying any site-specific behavior needed
	for i := range items {
		req, err := http.NewRequest(http.MethodGet, items[i].GetURL().String(), nil)
		if err != nil {
			logger.Error("unable to create request for URL", "url", items[i].GetURL().String(), "err", err.Error())
			items[i].SetStatus(models.ItemFailed)
			continue
		}

		// Apply configured User-Agent
		req.Header.Set("User-Agent", config.Get().UserAgent)

		switch {
		case tiktok.IsTikTokURL(items[i].GetURL()):
			tiktok.AddHeaders(req)
		}

		items[i].GetURL().SetRequest(req)
		items[i].SetStatus(models.ItemPreProcessed)
	}
}
