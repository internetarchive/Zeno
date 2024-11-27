// Zeno is a web crawler designed to operate wide crawls or to simply archive one web page.
// Zeno's key concepts are: portability, performance, simplicity ; with an emphasis on performance.

// Authors:
//
//	Corentin Barreau <corentin@archive.org>
//	Jake LaFountain <jakelf@archive.org>
//	Thomas Foubert <thomas@archive.org>
package main

import (
	"fmt"

	"github.com/internetarchive/Zeno/cmd"
)

func main() {
	if err := cmd.Run(); err != nil {
		fmt.Println(err.Error())
		return
	}
<<<<<<< HEAD
=======

	stats.Init()

	// If needed, create the seencheck DB (only if not using HQ)
	if config.Get().UseSeencheck && !config.Get().UseHQ {
		err := seencheck.Start(config.Get().JobPath)
		if err != nil {
			logger.Error("unable to start seencheck", "err", err.Error())
			return
		}
	}

	// Start the reactor that will receive
	reactorOutputChan := make(chan *models.Item)
	err := reactor.Start(config.Get().WorkersCount, reactorOutputChan)

	preprocessorOutputChan := make(chan *models.Item)
	err = preprocessor.Start(reactorOutputChan, preprocessorOutputChan)
	if err != nil {
		logger.Error("error starting preprocessor", "err", err.Error())
		return
	}

	archiverOutputChan := make(chan *models.Item)
	err = archiver.Start(preprocessorOutputChan, archiverOutputChan)
	if err != nil {
		logger.Error("error starting archiver", "err", err.Error())
		return
	}

	postprocessorOutputChan := make(chan *models.Item)
	err = postprocessor.Start(archiverOutputChan, postprocessorOutputChan)
	if err != nil {
		logger.Error("error starting postprocessor", "err", err.Error())
		return
	}

	var finisherFinishChan, finisherProduceChan chan *models.Item
	if config.Get().UseHQ {
		logger.Info("starting hq")

		finisherFinishChan = make(chan *models.Item)
		finisherProduceChan = make(chan *models.Item)

		err = hq.Start(finisherFinishChan, finisherProduceChan)
		if err != nil {
			logger.Error("error starting hq", "err", err.Error())
			return
		}
	}

	err = finisher.Start(postprocessorOutputChan, finisherFinishChan, finisherProduceChan)
	if err != nil {
		logger.Error("error starting finisher", "err", err.Error())
		return
	}

	// Pipe in the reactor the input seeds if any
	if len(config.Get().InputSeeds) > 0 {
		for _, seed := range config.Get().InputSeeds {
			item := models.NewItem(models.ItemSourceQueue)
			item.SetURL(&models.URL{Raw: seed})

			err = reactor.ReceiveInsert(item)
			if err != nil {
				logger.Error("unable to insert seed", "err", err.Error())
				return
			}
		}
	}

	// Handle OS signals for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-signalChan:
		logger.Info("received shutdown signal, stopping services...")
		// Catch a second signal to force exit
		go func() {
			<-signalChan
			logger.Info("received second shutdown signal, forcing exit...")
			os.Exit(1)
		}()
	}

	finisher.Stop()
	postprocessor.Stop()
	archiver.Stop()
	preprocessor.Stop()
	reactor.Freeze()
	hq.Stop()
	reactor.Stop()

	close(reactorOutputChan)
	close(preprocessorOutputChan)
	close(archiverOutputChan)
	close(postprocessorOutputChan)

	if finisherFinishChan != nil {
		close(finisherFinishChan)
	}

	if finisherProduceChan != nil {
		close(finisherProduceChan)
	}

	// Deleting temp directory (only if it's empty, else we keep it for debugging)
	// Note: it should NEVER contain anything
	if config.Get().WARCTempDir != "" {
		err := os.Remove(config.Get().WARCTempDir)
		if err != nil {
			logger.Error("unable to remove temp dir", "err", err.Error())
		}
	}

	logger.Info("all services stopped, exiting")
	return
>>>>>>> dev/v2
}
