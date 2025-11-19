// Package preprocessor is the stage of the pipeline that :
//
// 1. Checks that the received seed is consistent and has the correct status
// 2. Normalizes the seed's lowest level URLs
// 3. Checks if the URLs should be excluded
// 4. Removes any false-positive assets
// 5. Deduplicate the items
// 6. Seencheck the items
// 7. Builds the requests before handling them to the archiver
package preprocessor

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/log/dumper"
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor/sitespecific"
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

	Seenchecker    func(item *models.Item) error
	seencheckerSet bool
}

var (
	GlobalPreprocessor *preprocessor
	once               sync.Once
	logger             *log.FieldedLogger
)

// Start initializes the internal preprocessor structure and start routines, should only be called once and returns an error if called more than once
func Start(inputChan, outputChan chan *models.Item) error {
	logger = log.NewFieldedLogger(&log.Fields{
		"component": "preprocessor",
	})

	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		GlobalPreprocessor = &preprocessor{
			ctx:      ctx,
			cancel:   cancel,
			inputCh:  inputChan,
			outputCh: outputChan,
		}
		logger.Debug("initialized")
		for i := 0; i < config.Get().WorkersCount; i++ {
			GlobalPreprocessor.wg.Add(1)
			go GlobalPreprocessor.worker(strconv.Itoa(i))
		}
		logger.Info("started")
	})

	if GlobalPreprocessor == nil {
		return ErrPreprocessorAlreadyInitialized
	}

	return nil
}

// Stop stops the preprocessor routines
func Stop() {
	if GlobalPreprocessor != nil {
		GlobalPreprocessor.cancel()
		GlobalPreprocessor.wg.Wait()
		logger.Info("stopped")
	}
}

type SeencheckerFunc = func(item *models.Item) error

// SetSeenchecker sets the seenchecker function to be used by the preprocessor.
// It should be called only once, and it will panic if called more than once or if the preprocessor is not initialized.
func SetSeenchecker(seenchecker SeencheckerFunc) {
	if GlobalPreprocessor == nil {
		panic("preprocessor is not initialized")
	}

	if GlobalPreprocessor.seencheckerSet {
		panic("seenchecker is already set")
	}

	GlobalPreprocessor.Seenchecker = seenchecker
	GlobalPreprocessor.seencheckerSet = true
	logger.Debug("seenchecker set")
}

func (p *preprocessor) worker(workerID string) {
	defer p.wg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "preprocessor.worker",
		"worker_id": workerID,
	})

	defer logger.Debug("worker stopped")

	// Subscribe to the pause controler
	controlChans := pause.Subscribe()
	defer pause.Unsubscribe(controlChans)

	stats.PreprocessorRoutinesIncr()
	defer stats.PreprocessorRoutinesDecr()

	for {
		select {
		case <-p.ctx.Done():
			logger.Debug("shutting down")
			return
		case <-controlChans.PauseCh:
			logger.Debug("received pause event")
			controlChans.ResumeCh <- struct{}{}
			logger.Debug("received resume event")
		case seed, ok := <-p.inputCh:
			if ok {
				logger.Debug("received seed", "seed", seed.GetShortID())

				if err := seed.CheckConsistency(); err != nil {
					panic(fmt.Sprintf("seed consistency check failed with err: %s, seed id %s, worker_id %s", err.Error(), seed.GetShortID(), workerID))
				}

				if seed.GetStatus() == models.ItemFailed || seed.GetStatus() == models.ItemCompleted {
					panic(fmt.Sprintf("preprocessor received seed with status %d, seed id: %s, worker_id %s", seed.GetStatus(), seed.GetShortID(), workerID))
				}

				if err := preprocess(workerID, seed); err != nil {
					panic(fmt.Sprintf("preprocess failed with err: %v", err))
				}

				select {
				case <-p.ctx.Done():
					logger.Debug("aborting seed due to stop", "seed", seed.GetShortID())
					return
				case p.outputCh <- seed:
				}
			}
		}
	}
}

func preprocess(workerID string, seed *models.Item) error {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "preprocessor.preprocess",
		"worker_id": workerID,
		"seed_id":   seed.GetShortID(),
	})

	operatingDepth := seed.GetMaxDepth()

	items, err := seed.GetNodesAtLevel(operatingDepth)
	if err != nil {
		return err
	}

	for i := range items {
		// Panic on any child that is not fresh
		// This means that an incorrect item was inserted and/or that the finisher is not working correctly
		if items[i].GetStatus() != models.ItemFresh {
			dumper.PanicWithDump(fmt.Sprintf("non-fresh item %s received in preprocessor worker %s with status: %s", items[i].GetShortID(), workerID, items[i].GetStatus()), items[i])
		}

		// Normalize the URL
		if items[i].IsSeed() {
			err := NormalizeURL(items[i].GetURL(), nil)
			if err != nil {
				logger.Debug("unable to validate URL", "item_id", items[i].GetShortID(), "url", items[i].GetURL().Raw, "err", err.Error())
				items[i].SetStatus(models.ItemFailed)
				return nil
			}
		} else {
			err := NormalizeURL(items[i].GetURL(), items[i].GetParent().GetURL())
			if err != nil {
				logger.Debug("unable to validate URL", "item_id", items[i].GetShortID(), "url", items[i].GetURL().Raw, "err", err.Error())
				items[i].GetParent().RemoveChild(items[i])
				continue
			}
		}

		// Apply include filters first, if any are defined
		if len(config.Get().IncludeHosts) > 0 || len(config.Get().IncludeString) > 0 {
			if !utils.StringContainsSliceElements(items[i].GetURL().GetParsed().Host, config.Get().IncludeHosts) &&
				!utils.StringContainsSliceElements(items[i].GetURL().String(), config.Get().IncludeString) {

				logger.Debug("URL excluded (does not match include filters)",
					"item_id", items[i].GetShortID(),
					"url", items[i].GetURL())

				if items[i].IsChild() || items[i].IsRedirection() {
					items[i].GetParent().RemoveChild(items[i])
					continue
				}

				items[i].SetStatus(models.ItemCompleted)
				return nil
			}
		}

		// Apply exclusion filters even if it passed inclusion
		if utils.StringContainsSliceElements(items[i].GetURL().GetParsed().Host, config.Get().ExcludeHosts) ||
			utils.StringContainsSliceElements(items[i].GetURL().String(), config.Get().ExcludeString) ||
			matchRegexExclusion(config.Get().GetExclusionRegexes(), items[i]) {

			logger.Debug("URL excluded (matches exclusion filters)",
				"item_id", items[i].GetShortID(),
				"url", items[i].GetURL())

			if items[i].IsChild() || items[i].IsRedirection() {
				items[i].GetParent().RemoveChild(items[i])
				continue
			}

			items[i].SetStatus(models.ItemCompleted)
			return nil
		}

		// If we are processing assets, then we need to remove childs that are just domains
		// (which means that they are not assets, but false positives)
		if items[i].IsChild() {
			if items[i].GetURL().GetParsed().Path == "" || items[i].GetURL().GetParsed().Path == "/" {
				logger.Debug("removing child with empty path", "item_id", items[i].GetShortID(), "url", items[i].GetURL().Raw)
				items[i].GetParent().RemoveChild(items[i])
			}
		}
	}

	// Deduplicate items based on their URL and remove duplicates
	seed.DedupeItems()

	items, err = seed.GetNodesAtLevel(operatingDepth)
	if err != nil {
		return err
	}

	if len(items) == 0 {
		logger.Debug("no more work to do after dedupe")
		seed.SetStatus(models.ItemCompleted)
		return nil
	}

	// If the item is a redirection or an asset, we need to seencheck it if needed
	if (config.Get().UseHQ || config.Get().UseSeencheck) && GlobalPreprocessor.seencheckerSet {
		var err error
		for i := 0; i < 5; i++ {
			err = GlobalPreprocessor.Seenchecker(seed)
			if err == nil {
				break
			}
			time.Sleep(1 * time.Second)
		}

		if err != nil {
			logger.Error("unable to seencheck seed", "err", err.Error())
			return err
		}
	}

	// Recreate the items list after deduplication and seencheck
	items, err = seed.GetNodesAtLevel(operatingDepth)
	if err != nil {
		return err
	}

	// Remove any item that is not fresh from the list
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].GetStatus() != models.ItemFresh {
			items = append(items[:i], items[i+1:]...)
		}
	}

	if len(items) == 0 {
		logger.Debug("no more work to do after seencheck")
		seed.SetStatus(models.ItemCompleted)
		return nil
	}

	// Finally, we build the requests, applying any site-specific behavior needed
	for i := range items {
		req, err := http.NewRequest(http.MethodGet, items[i].GetURL().String(), nil)
		if err != nil {
			logger.Error("unable to create request for URL", "item_id", items[i].GetShortID(), "url", items[i].GetURL(), "err", err.Error())
			items[i].SetStatus(models.ItemFailed)
			continue
		}

		// Apply configured User-Agent
		req.Header.Set("User-Agent", config.Get().UserAgent)

		sitespecific.RunPreprocessors(items[i].GetURL(), req)

		items[i].GetURL().SetRequest(req)
		items[i].SetStatus(models.ItemPreProcessed)
	}
	return nil
}
