package postprocessor

import (
	"context"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/internal/pkg/config"
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

	for {
		select {
		// Closes the run routine when context is canceled
		case <-globalPostprocessor.ctx.Done():
			logger.Debug("shutting down")
			wg.Wait()
			return
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

					postprocess(item)

					select {
					case <-ctx.Done():
						return
					case globalPostprocessor.outputCh <- item:
					}
				}(ctx)
			}
		}
	}
}

func postprocess(item *models.Item) {
	// if item.GetStatus() != models.ItemFailed {
	// 	item.SetRedirection(nil)
	// 	return
	// }

	defer item.SetStatus(models.ItemPostProcessed)

	var (
		URLs    []*models.URL
		URLType models.URLType
	)

	if item.GetRedirection() != nil {
		URLType = models.URLTypeRedirection
		URLs = append(URLs, item.GetRedirection())
	} else if len(item.GetChilds()) > 0 {
		URLType = models.URLTypeAsset
		URLs = item.GetChilds()
		item.IncrChildsHops()
		item.SetChilds(nil)
	} else {
		URLType = models.URLTypeSeed
		URLs = append(URLs, item.GetURL())
	}

	for _, URL := range URLs {
		// Verify if there is any redirection
		// TODO: execute assets redirection
		if URLType == models.URLTypeSeed && isStatusCodeRedirect(URL.GetResponse().StatusCode) {
			// Check if the current redirections count doesn't exceed the max allowed
			if URL.GetRedirects() >= config.Get().MaxRedirect {
				logger.Warn("max redirects reached", "item", item.GetShortID())
				return
			}

			// Prepare the new item resulting from the redirection
			item.SetRedirection(&models.URL{
				Raw:       URL.GetResponse().Header.Get("Location"),
				Redirects: URL.GetRedirects() + 1,
				Hops:      URL.GetHops(),
			})

			return
		} else {
			item.SetRedirection(nil)
		}

		if item.GetChildsHops() > 1 ||
			config.Get().DisableAssetsCapture && !config.Get().DomainsCrawl && (uint64(config.Get().MaxHops) <= uint64(URL.GetHops())) {
			return
		}

		if URL.GetResponse() != nil {
			// Generate the goquery document from the response body
			doc, err := goquery.NewDocumentFromReader(URL.GetBody())
			if err != nil {
				logger.Error("unable to create goquery document", "err", err.Error(), "item", item.GetShortID())
				return
			}

			URL.RewindBody()

			// If the URL is a seed, scrape the base tag
			if URLType == models.URLTypeSeed {
				scrapeBaseTag(doc, item)
			}

			// Extract assets from the document
			err = extractAssets(doc, URL, item)
			if err != nil {
				logger.Error("unable to extract assets", "err", err.Error(), "item", item.GetShortID(), "func", "postprocessor.postprocess")
			}
		}
	}
}
