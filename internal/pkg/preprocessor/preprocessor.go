package preprocessor

import (
	"context"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor/seencheck"
	"github.com/internetarchive/Zeno/internal/pkg/source/hq"
	"github.com/internetarchive/Zeno/pkg/models"
)

type preprocessor struct {
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	input  chan *models.Item
	output chan *models.Item
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

	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		globalPreprocessor = &preprocessor{
			ctx:    ctx,
			cancel: cancel,
			input:  inputChan,
			output: outputChan,
		}
		globalPreprocessor.wg.Add(1)
		go globalPreprocessor.run()
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
		close(globalPreprocessor.output)
		logger.Info("stopped")
	}
}

func (p *preprocessor) run() {
	defer p.wg.Done()

	var (
		wg    sync.WaitGroup
		guard = make(chan struct{}, config.Get().WorkersCount)
	)

	for {
		select {
		// Closes the run routine when context is canceled
		case <-p.ctx.Done():
			logger.Info("shutting down")
			return
		case item, ok := <-p.input:
			if ok {
				guard <- struct{}{}
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() { <-guard }()
					p.preprocess(item)
				}()
			}
		}
	}
}

func (p *preprocessor) preprocess(item *models.Item) {
	// Validate the URL of either the item itself and/or its childs
	// TODO: if an error happen and it's a fresh item, we should mark it as failed in HQ (if it's a HQ-based crawl)

	var (
		err             error
		URLsToSeencheck []*models.URL
		URLType         string
	)

	// Validate the URLs, either the item's URL or its childs if it has any
	if item.Status == models.ItemFresh {
		URLType = "seed"

		// Validate the item's URL itself
		err = validateURL(item.URL, nil)
		if err != nil {
			logger.Warn("unable to validate URL", "url", item.URL.Raw, "err", err.Error(), "func", "preprocessor.preprocess")
			return
		}

		if config.Get().UseSeencheck {
			URLsToSeencheck = append(URLsToSeencheck, item.URL)
		}
	} else if len(item.Childs) > 0 {
		URLType = "asset"

		// Validate the URLs of the child items
		for i := 0; i < len(item.Childs); {
			err = validateURL(item.Childs[i], item.URL)
			if err != nil {
				// If we can't validate an URL, we remove it from the list of childs
				logger.Warn("unable to validate URL", "url", item.Childs[i].Raw, "err", err.Error(), "func", "preprocessor.preprocess")
				item.Childs = append(item.Childs[:i], item.Childs[i+1:]...)
			} else {
				if config.Get().UseSeencheck {
					URLsToSeencheck = append(URLsToSeencheck, item.Childs[i])
				}

				i++
			}
		}
	} else {
		logger.Error("item got into preprocessing without anything to preprocess")
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

	// Final step, send the preprocessed item to the output chan of the preprocessor
	p.output <- item
}
