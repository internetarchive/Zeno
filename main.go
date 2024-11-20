// Zeno is a web crawler designed to operate wide crawls or to simply archive one web page.
// Zeno's key concepts are: portability, performance, simplicity ; with an emphasis on performance.

// Authors:
//
//	Corentin Barreau <corentin@archive.org>
//	Jake LaFountain <jakelf@archive.org>
//	Thomas Foubert <thomas@archive.org>
package main

import (
	"time"

	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/cmd"
	"github.com/internetarchive/Zeno/internal/pkg/archiver"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/finisher"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor"
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor"
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor/seencheck"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
	"github.com/internetarchive/Zeno/pkg/models"
)

var (
	logger *log.FieldedLogger
)

func main() {
	log.Start()
	logger = log.NewFieldedLogger(&log.Fields{
		"component": "main",
	})
	defer log.Stop()

	if err := cmd.Run(); err != nil {
		logger.Error("unable to run root command", "err", err.Error())
		return
	}

	// If needed, start the seencheck process
	if config.Get().UseSeencheck {
		err := seencheck.Start(config.Get().JobPath)
		if err != nil {
			logger.Error("unable to start seencheck", "err", err.Error())
			return
		}
	}

	seedErrorChan := make(chan *models.Item)

	// Start the reactor that will receive
	reactorOutputChan := make(chan *models.Item)
	err := reactor.Start(config.Get().WorkersCount, reactorOutputChan)

	// Create mock seeds
	mockItems := make([]*models.Item, 1)
	URL := "http://www.youtube.com/watch?v=stUqfrc1EFE"
	UUID := uuid.New()

	mockItems[0] = &models.Item{
		ID:     UUID.String(),
		URL:    &models.URL{Raw: URL},
		Status: models.ItemFresh,
		Source: models.ItemSourceHQ,
	}

	preprocessorOutputChan := make(chan *models.Item)
	err = preprocessor.Start(reactorOutputChan, preprocessorOutputChan, seedErrorChan)
	if err != nil {
		logger.Error("error starting preprocessor", "err", err.Error())
		return
	}

	archiverOutputChan := make(chan *models.Item)
	err = archiver.Start(preprocessorOutputChan, archiverOutputChan, seedErrorChan)
	if err != nil {
		logger.Error("error starting archiver", "err", err.Error())
		return
	}

	postprocessorOutputChan := make(chan *models.Item)
	err = postprocessor.Start(archiverOutputChan, postprocessorOutputChan, seedErrorChan)
	if err != nil {
		logger.Error("error starting postprocessor", "err", err.Error())
		return
	}

	err = finisher.Start(postprocessorOutputChan, seedErrorChan)
	if err != nil {
		logger.Error("error starting finisher", "err", err.Error())
		return
	}

	// Queue mock seeds to the source channel
	for _, seed := range mockItems {
		err := reactor.ReceiveInsert(seed)
		if err != nil {
			logger.Error("Error queuing seed to source channel", "err", err.Error())
			return
		}
	}

	for {
		time.Sleep(1 * time.Second)
		if len(reactor.GetStateTable()) == 0 {
			for archiver.GetWARCWritingQueueSize() != 0 {
				logger.Info("waiting for WARC client(s) to finish writing to disk", "queue_size", archiver.GetWARCWritingQueueSize())
			}

			finisher.Stop()
			postprocessor.Stop()
			archiver.Stop()
			preprocessor.Stop()
			reactor.Stop()
			return
		}
	}
}
