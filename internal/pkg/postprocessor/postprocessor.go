package postprocessor

import (
	"context"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
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

					if item.GetStatus() != models.ItemFailed {
						postprocess(item)
					}

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
	// If we don't capture assets, there is no need to postprocess the item
	// TODO: handle hops even with disable assets capture
	if config.Get().DisableAssetsCapture {
		return
	}

	items, err := item.GetNodesAtLevel(item.GetMaxDepth())
	if err != nil {
		logger.Error("unable to get nodes at level", "err", err.Error(), "item", item.GetShortID())
		panic(err)
	}

	for _, i := range items {
		// Verify if there is any redirection
		// TODO: execute assets redirection
		if isStatusCodeRedirect(i.GetURL().GetResponse().StatusCode) {
			// Check if the current redirections count doesn't exceed the max allowed
			if i.GetURL().GetRedirects() >= config.Get().MaxRedirect {
				logger.Warn("max redirects reached", "item", item.GetShortID(), "func", "postprocessor.postprocess")
				return
			}

			// Prepare the new item resulting from the redirection
			newURL := &models.URL{
				Raw:       i.GetURL().GetResponse().Header.Get("Location"),
				Redirects: i.GetURL().GetRedirects() + 1,
				Hops:      i.GetURL().GetHops(),
			}

			i.SetStatus(models.ItemGotRedirected)
			i.AddChild(models.NewItem(uuid.New().String(), newURL, "", false), i.GetStatus())

			return
		}

		// Return if:
		// - the item is a child and the URL has more than one hop
		// - assets capture is disabled and domains crawl is disabled
		// - the URL has more hops than the max allowed
		if (i.IsChild() && i.GetURL().GetHops() > 1) ||
			config.Get().DisableAssetsCapture && !config.Get().DomainsCrawl && (uint64(config.Get().MaxHops) <= uint64(i.GetURL().GetHops())) {
			return
		}

		if i.GetURL().GetResponse() != nil {
			// Generate the goquery document from the response body
			doc, err := goquery.NewDocumentFromReader(i.GetURL().GetBody())
			if err != nil {
				logger.Error("unable to create goquery document", "err", err.Error(), "item", i.GetShortID())
				return
			}

			i.GetURL().RewindBody()

			// If the URL is a seed, scrape the base tag
			if i.IsSeed() || i.IsRedirection() {
				scrapeBaseTag(doc, i)
			}

			// Extract assets from the document
			assets, err := extractAssets(doc, i.GetURL(), i)
			if err != nil {
				logger.Error("unable to extract assets", "err", err.Error(), "item", i.GetShortID())
			}

			for _, asset := range assets {
				if assets == nil {
					logger.Warn("nil asset", "item", i.GetShortID())
					continue
				}

				i.SetStatus(models.ItemGotChildren)
				i.AddChild(models.NewItem(uuid.New().String(), asset, "", false), i.GetStatus())
			}
		}

		if i.GetStatus() != models.ItemGotChildren && i.GetStatus() != models.ItemGotRedirected {
			i.SetStatus(models.ItemCompleted)
		}
	}
}
