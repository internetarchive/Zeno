package preprocessor

import (
	"context"
	"net/http"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/config"
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
	errorCh  chan *models.Item
}

var (
	globalPreprocessor *preprocessor
	once               sync.Once
	logger             *log.FieldedLogger
)

// This functions starts the preprocessor responsible for preparing
// the seeds sent by the reactor for captures
func Start(inputChan, outputChan, errorChan chan *models.Item) error {
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
			errorCh:  errorChan,
		}
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
		close(globalPreprocessor.outputCh)
		logger.Info("stopped")
	}
}

func run() {
	defer globalPreprocessor.wg.Done()

	var (
		wg    sync.WaitGroup
		guard = make(chan struct{}, config.Get().WorkersCount)
	)

	for {
		select {
		// Closes the run routine when context is canceled
		case <-globalPreprocessor.ctx.Done():
			logger.Info("shutting down")
			return
		case item, ok := <-globalPreprocessor.inputCh:
			if ok {
				logger.Info("received item", "item", item.ID)
				guard <- struct{}{}
				wg.Add(1)
				stats.PreprocessorRoutinesIncr()
				go func() {
					defer wg.Done()
					defer func() { <-guard }()
					defer stats.PreprocessorRoutinesDecr()
					preprocess(item)
					globalPreprocessor.outputCh <- item
				}()
			}
		}
	}
}

func preprocess(item *models.Item) {
	defer item.SetStatus(models.ItemPreProcessed)

	// Validate the URL of either the item itself and/or its childs
	// TODO: if an error happen and it's a fresh item, we should mark it as failed in HQ (if it's a HQ-based crawl)

	var (
		err             error
		URLsToSeencheck []*models.URL
		URLType         string
	)

	// Validate the URLs, either the item's URL or its childs if it has any
	if item.GetStatus() == models.ItemFresh {
		URLType = "seed"

		// Validate the item's URL itself
		err = normalizeURL(item.URL, nil)
		if err != nil {
			logger.Warn("unable to validate URL", "url", item.URL.Raw, "err", err.Error(), "func", "preprocessor.preprocessor")
			return
		}

		if config.Get().UseSeencheck {
			URLsToSeencheck = append(URLsToSeencheck, item.URL)
		}
	} else if item.GetRedirection() != nil {
		URLType = "seed"

		// Validate the item's URL itself
		err = normalizeURL(item.GetURL(), nil)
		if err != nil {
			logger.Warn("unable to validate URL", "url", item.URL.Raw, "err", err.Error(), "func", "preprocessor.preprocessor")
			return
		}

		if config.Get().UseSeencheck {
			URLsToSeencheck = append(URLsToSeencheck, item.URL)
		}
	} else if len(item.Childs) > 0 {
		URLType = "asset"

		// Validate the URLs of the child items
		for i := 0; i < len(item.Childs); {
			err = normalizeURL(item.Childs[i], item.URL)
			if err != nil {
				// If we can't validate an URL, we remove it from the list of childs
				logger.Warn("unable to validate URL", "url", item.Childs[i].Raw, "err", err.Error(), "func", "preprocessor.preprocessor")
				item.Childs = append(item.Childs[:i], item.Childs[i+1:]...)
			} else {
				if config.Get().UseSeencheck {
					URLsToSeencheck = append(URLsToSeencheck, item.Childs[i])
				}

				i++
			}
		}
	} else {
		logger.Error("item got into preprocessoring without anything to preprocessor")
	}

	// If we have URLs to seencheck, we do it
	if len(URLsToSeencheck) > 0 {
		var seencheckedURLs []*models.URL

		if config.Get().HQ {
			seencheckedURLs, err = hq.SeencheckURLs(URLType, item.URL)
			if err != nil {
				logger.Warn("unable to seencheck URL", "url", item.URL.Raw, "err", err.Error(), "func", "preprocessor.preprocess")
				return
			}
		} else {
			seencheckedURLs, err = seencheck.SeencheckURLs(URLType, item.URL)
			if err != nil {
				logger.Warn("unable to seencheck URL", "url", item.URL.Raw, "err", err.Error(), "func", "preprocessor.preprocess")
				return
			}
		}

		if len(seencheckedURLs) == 0 {
			return
		}

		if URLType == "seed" {
			item.URL = seencheckedURLs[0]
		} else {
			item.Childs = seencheckedURLs
		}
	}

	// Finally, we build the requests, applying any site-specific behavior needed
	if URLType == "seed" {
		// TODO: apply site-specific stuff
		req, err := http.NewRequest(http.MethodGet, item.URL.String(), nil)
		if err != nil {
			logger.Error("unable to create new request for URL", "url", item.URL.String(), "err", err.Error(), "func", "preprocessor.preprocess")
			return
		}

		item.URL.SetRequest(req)
	} else {
		for i, child := range item.Childs {
			// TODO: apply site-specific stuff
			req, err := http.NewRequest(http.MethodGet, child.String(), nil)
			if err != nil {
				logger.Error("unable to create new request for URL", "url", item.URL.String(), "err", err.Error(), "func", "preprocessor.preprocess")
				return
			}

			item.Childs[i].SetRequest(req)
		}
	}
}
