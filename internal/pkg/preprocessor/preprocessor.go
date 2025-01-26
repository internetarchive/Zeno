package preprocessor

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/log/dumper"
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor/seencheck"
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor/sitespecific/tiktok"
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor/sitespecific/truthsocial"
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

					if err := item.CheckConsistency(); err != nil {
						panic(fmt.Sprintf("item consistency check failed with err: %s, item id %s", err.Error(), item.GetShortID()))
					}

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

	operatingDepth := item.GetMaxDepth()

	children, err := item.GetNodesAtLevel(operatingDepth)
	if err != nil {
		panic(err)
	}

	for i := range children {
		// Panic on any child that is not fresh
		// This means that an incorrect item was inserted and/or that the finisher is not working correctly
		if children[i].GetStatus() != models.ItemFresh {
			dumper.Dump(item)
			panic(fmt.Sprintf("non-fresh item %s received in preprocessor: %s", children[i].GetShortID(), children[i].GetStatus().String()))
		}

		// Normalize the URL
		if children[i].IsSeed() {
			err := normalizeURL(children[i].GetURL(), nil)
			if err != nil {
				logger.Debug("unable to validate URL", "item_id", children[i].GetShortID(), "url", children[i].GetURL().Raw, "err", err.Error())
				children[i].SetStatus(models.ItemFailed)
				return
			}
		} else {
			err := normalizeURL(children[i].GetURL(), children[i].GetParent().GetURL())
			if err != nil {
				logger.Debug("unable to validate URL", "item_id", children[i].GetShortID(), "url", children[i].GetURL().Raw, "err", err.Error())
				children[i].GetParent().RemoveChild(children[i])
				continue
			}
		}

		// Verify if the URL isn't to be excluded
		if utils.StringContainsSliceElements(children[i].GetURL().GetParsed().Host, config.Get().ExcludeHosts) ||
			utils.StringContainsSliceElements(children[i].GetURL().GetParsed().Path, config.Get().ExcludeString) ||
			matchRegexExclusion(children[i]) {
			logger.Debug("URL excluded", "item_id", children[i].GetShortID(), "url", children[i].GetURL().String())
			if children[i].IsChild() || children[i].IsRedirection() {
				children[i].GetParent().RemoveChild(children[i])
				continue
			}

			children[i].SetStatus(models.ItemCompleted)
			return
		}

		// If we are processing assets, then we need to remove childs that are just domains
		// (which means that they are not assets, but false positives)
		if children[i].IsChild() {
			if children[i].GetURL().GetParsed().Path == "" || children[i].GetURL().GetParsed().Path == "/" {
				logger.Debug("removing child with empty path", "item_id", children[i].GetShortID(), "url", children[i].GetURL().Raw)
				children[i].GetParent().RemoveChild(children[i])
			}
		}
	}

	// Deduplicate items based on their URL and remove duplicates
	item.DedupeItems()

	children, err = item.GetNodesAtLevel(operatingDepth)
	if err != nil {
		panic(err)
	}

	if len(children) == 0 {
		logger.Info("no more work to do after dedupe", "item_id", item.GetShortID())
		item.SetStatus(models.ItemCompleted)
		return
	}

	// If the item is a redirection or an asset, we need to seencheck it if needed
	if config.Get().UseHQ {
		err = hq.SeencheckItem(item)
		if err != nil {
			logger.Warn("unable to seencheck item", "item_id", item.GetShortID(), "err", err.Error(), "func", "preprocessor.preprocess")
		}
	} else {
		err = seencheck.SeencheckItem(item)
		if err != nil {
			logger.Warn("unable to seencheck item", "item_id", item.GetShortID(), "err", err.Error(), "func", "preprocessor.preprocess")
		}
	}

	// Recreate the items list after deduplication and seencheck
	children, err = item.GetNodesAtLevel(operatingDepth)
	if err != nil {
		panic(err)
	}

	// Remove any item that is not fresh from the list
	for i := len(children) - 1; i >= 0; i-- {
		if children[i].GetStatus() != models.ItemFresh {
			children = append(children[:i], children[i+1:]...)
		}
	}

	if len(children) == 0 {
		logger.Info("no more work to do after seencheck", "item_id", item.GetShortID())
		item.SetStatus(models.ItemCompleted)
		return
	}

	// Finally, we build the requests, applying any site-specific behavior needed
	for i := range children {
		req, err := http.NewRequest(http.MethodGet, children[i].GetURL().String(), nil)
		if err != nil {
			logger.Error("unable to create request for URL", "item_id", children[i].GetShortID(), "url", children[i].GetURL().String(), "err", err.Error())
			children[i].SetStatus(models.ItemFailed)
			continue
		}

		// Apply configured User-Agent
		req.Header.Set("User-Agent", config.Get().UserAgent)

		switch {
		case tiktok.IsTikTokURL(children[i].GetURL()):
			tiktok.AddHeaders(req)
		case truthsocial.IsStatusAPIURL(children[i].GetURL()) || truthsocial.IsVideoAPIURL(children[i].GetURL()):
			truthsocial.AddStatusAPIHeaders(req)
		case truthsocial.IsAccountsAPIURL(children[i].GetURL()):
			truthsocial.AddAccountsAPIHeaders(req)
		}

		children[i].GetURL().SetRequest(req)
		children[i].SetStatus(models.ItemPreProcessed)
	}
}
