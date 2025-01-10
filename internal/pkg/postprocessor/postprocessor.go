package postprocessor

import (
	"context"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/sitespecific/facebook"
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

func postprocess(item *models.Item) (outlinks []*models.Item) {
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

	for i := range items {
		if items[i].GetStatus() != models.ItemArchived {
			logger.Debug("item not archived, skipping", "item", items[i].GetShortID())
			continue
		}

		// Verify if there is any redirection
		// TODO: execute assets redirection
		if isStatusCodeRedirect(items[i].GetURL().GetResponse().StatusCode) {
			// Check if the current redirections count doesn't exceed the max allowed
			if items[i].GetURL().GetRedirects() >= config.Get().MaxRedirect {
				logger.Warn("max redirects reached", "item", item.GetShortID(), "func", "postprocessor.postprocess")
				items[i].SetStatus(models.ItemCompleted)
				continue
			}

			// Prepare the new item resulting from the redirection
			newURL := &models.URL{
				Raw:       items[i].GetURL().GetResponse().Header.Get("Location"),
				Redirects: items[i].GetURL().GetRedirects() + 1,
				Hops:      items[i].GetURL().GetHops(),
			}

			items[i].SetStatus(models.ItemGotRedirected)
			err := items[i].AddChild(models.NewItem(uuid.New().String(), newURL, "", false), items[i].GetStatus())
			if err != nil {
				panic(err)
			}

			continue
		}

		// Execute site-specific post-processing
		switch {
		case facebook.IsFacebookPostURL(items[i].GetURL()):
			err := items[i].AddChild(
				models.NewItem(
					uuid.New().String(),
					facebook.GenerateEmbedURL(items[i].GetURL()),
					items[i].GetURL().String(),
					true,
				), models.ItemGotChildren)
			if err != nil {
				panic(err)
			}
		}

		// Return if:
		// - the item is a child and the URL has more than one hop
		// - assets capture is disabled and domains crawl is disabled
		// - the URL has more hops than the max allowed
		if (items[i].IsChild() && items[i].GetURL().GetHops() > 1) || config.Get().DisableAssetsCapture && !config.Get().DomainsCrawl && (uint64(config.Get().MaxHops) <= uint64(items[i].GetURL().GetHops())) {
			items[i].SetStatus(models.ItemCompleted)
			continue
		}

		if items[i].GetURL().GetResponse() != nil {
			// Generate the goquery document from the response body
			doc, err := goquery.NewDocumentFromReader(items[i].GetURL().GetBody())
			if err != nil {
				logger.Error("unable to create goquery document", "err", err.Error(), "item", items[i].GetShortID())
				continue
			}

			items[i].GetURL().RewindBody()

			// If the URL is a seed, scrape the base tag
			if items[i].IsSeed() || items[i].IsRedirection() {
				scrapeBaseTag(doc, items[i])
			}

			// Extract assets from the document
			if !config.Get().DisableAssetsCapture {
				assets, err := extractAssets(doc, items[i].GetURL(), items[i])
				if err != nil {
					logger.Error("unable to extract assets", "err", err.Error(), "item", items[i].GetShortID())
				}

				for _, asset := range assets {
					if assets == nil {
						logger.Warn("nil asset", "item", items[i].GetShortID())
						continue
					}

					items[i].SetStatus(models.ItemGotChildren)
					items[i].AddChild(models.NewItem(uuid.New().String(), asset, "", false), items[i].GetStatus())
				}
			}

			// Extract outlinks from the page
			if config.Get().DomainsCrawl || ((items[i].IsSeed() || items[i].IsRedirection()) && items[i].GetURL().GetHops() < config.Get().MaxHops) {
				logger.Info("extracting outlinks", "item", items[i].GetShortID())
				links, err := extractOutlinks(items[i].GetURL(), items[i])
				if err != nil {
					logger.Error("unable to extract outlinks", "err", err.Error(), "item", items[i].GetShortID())
					continue
				}

				for _, link := range links {
					if link == nil {
						logger.Warn("nil link", "item", items[i].GetShortID())
						continue
					}

					outlinks = append(outlinks, models.NewItem(uuid.New().String(), link, items[i].GetURL().String(), true))
				}

				logger.Debug("extracted outlinks", "item", items[i].GetShortID(), "count", len(links))
			}
		}

		if items[i].GetStatus() != models.ItemGotChildren && items[i].GetStatus() != models.ItemGotRedirected {
			items[i].SetStatus(models.ItemCompleted)
		}
	}

	return
}
