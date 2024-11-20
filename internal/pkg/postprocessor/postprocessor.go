package postprocessor

import (
	"context"
	"sync"

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
	errorCh  chan *models.Item
}

var (
	globalPostprocessor *postprocessor
	once                sync.Once
	logger              *log.FieldedLogger
)

// This functions starts the preprocessor responsible for preparing
// the seeds sent by the reactor for captures
func Start(inputChan, outputChan, errorChan chan *models.Item) error {
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
			errorCh:  errorChan,
		}
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
		close(globalPostprocessor.outputCh)
		logger.Info("stopped")
	}
}

func run() {
	defer globalPostprocessor.wg.Done()

	var (
		wg    sync.WaitGroup
		guard = make(chan struct{}, config.Get().WorkersCount)
	)

	for {
		select {
		// Closes the run routine when context is canceled
		case <-globalPostprocessor.ctx.Done():
			logger.Info("shutting down")
			return
		case item, ok := <-globalPostprocessor.inputCh:
			if ok {
				logger.Info("received item", "item", item.ID)
				guard <- struct{}{}
				wg.Add(1)
				stats.PostprocessorRoutinesIncr()
				go func() {
					defer wg.Done()
					defer func() { <-guard }()
					defer stats.PostprocessorRoutinesDecr()
					postprocess(item)
					globalPostprocessor.outputCh <- item
				}()
			}
		}
	}
}

func postprocess(item *models.Item) {
	// Verify if there is any redirection
	if isStatusCodeRedirect(item.URL.GetResponse().StatusCode) {
		// Check if the current redirections count doesn't exceed the max allowed
		if item.URL.GetRedirects() >= config.Get().MaxRedirect {
			logger.Warn("max redirects reached", "item", item.ID)
			item.Status = models.ItemCanceled
			return
		}

		// Prepare the new item resulting from the redirection
		item.Redirection = &models.URL{
			Raw:       item.URL.GetResponse().Header.Get("Location"),
			Redirects: item.URL.GetRedirects() + 1,
			Hops:      item.URL.GetHops(),
		}

		return
	}
}
