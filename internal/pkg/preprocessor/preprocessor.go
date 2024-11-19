package preprocessor

import (
	"context"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
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

func init() {
	log.Init()
	logger = log.NewFieldedLogger(&log.Fields{
		"component": "preprocessor",
	})
}

// This functions starts the preprocessor responsible for preparing
// the seeds sent by the reactor for captures
func Start(inputChan, outputChan chan *models.Item) error {
	var done bool

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
	var err error
	if item.Status == models.ItemFresh {
		// Preprocess the item's URL itself
		item.URL.Value, err = validateURL(item.URL.Value, nil)
		if err != nil {
			logger.Warn("unable to validate URL", "url", item.URL.Value, "err", err.Error(), "func", "preprocessor.preprocess")
			return
		}
	} else if len(item.Childs) > 0 {
		// Preprocess the childs
		for i := 0; i < len(item.Childs); {
			child := item.Childs[i]
			item.Childs[i].Value, err = validateURL(child.Value, item.URL)
			if err != nil {
				// If we can't validate an URL, we remove it from the list of childs
				logger.Warn("unable to validate URL", "url", child.Value, "err", err.Error(), "func", "preprocessor.preprocess")
				item.Childs = append(item.Childs[:i], item.Childs[i+1:]...)
			} else {
				i++
			}
		}
	} else {
		logger.Error("item got into preprocessing without anything to preprocess")
	}

	// Final step, send the preprocessed item to the output chan of the preprocessor
	p.output <- item
}
