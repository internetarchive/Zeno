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
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/cmd"
	"github.com/internetarchive/Zeno/internal/pkg/archiver"
	"github.com/internetarchive/Zeno/internal/pkg/finisher"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor"
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/internetarchive/Zeno/pkg/models"
)

var (
	logger *log.FieldedLogger
)

func main() {
	log.Start()
	logger = log.NewFieldedLogger(&log.Fields{
		"component": "preprocessor",
	})
	defer log.Stop()

	if err := cmd.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	seedErrorChan := make(chan *models.Item)

	// Start the reactor that will receive
	reactorOutputChan := make(chan *models.Item)
	// err := reactor.Start(config.Get().WorkersCount, reactorOutputChan)
	err := reactor.Start(5, reactorOutputChan)
	if err != nil {
		logger.Error("error starting reactor", "err", err.Error())
		return
	}
	defer reactor.Stop()

	// Create mock seeds
	seeds := 5
	mockItems := []*models.Item{}
	for i := 0; i <= seeds; i++ {
		uuid := uuid.New()
		mockItems = append(mockItems, &models.Item{
			UUID:   &uuid,
			URL:    &models.URL{Raw: fmt.Sprintf("https://www.deezer.fr/%d", i)},
			Status: models.ItemFresh,
			Source: models.ItemSourceHQ,
		})
	}

	preprocessorOutputChan := make(chan *models.Item)
	err = preprocessor.Start(reactorOutputChan, preprocessorOutputChan, seedErrorChan)
	if err != nil {
		logger.Error("error starting preprocessor", "err", err.Error())
		return
	}
	defer preprocessor.Stop()

	archiverOutputChan := make(chan *models.Item)
	err = archiver.Start(preprocessorOutputChan, archiverOutputChan, seedErrorChan)
	if err != nil {
		logger.Error("error starting archiver", "err", err.Error())
		return
	}
	defer archiver.Stop()

	postprocessorOutputChan := make(chan *models.Item)
	err = postprocessor.Start(archiverOutputChan, postprocessorOutputChan, seedErrorChan)
	if err != nil {
		logger.Error("error starting postprocessor", "err", err.Error())
		return
	}
	defer postprocessor.Stop()

	err = finisher.Start(postprocessorOutputChan, seedErrorChan)
	if err != nil {
		logger.Error("error starting finisher", "err", err.Error())
		return
	}

	// Queue mock seeds to the source channel
	for _, seed := range mockItems {
		err := reactor.ReceiveInsert(seed)
		if err != nil {
			logger.Error("Error queuing seed to source channel", "error", err.Error())
			return
		}
	}

	for {
		time.Sleep(1 * time.Second)
		if len(reactor.GetStateTable()) == 0 {
			return
		}
		fmt.Println("URLsCrawledGet" + string(stats.URLsCrawledGet()))
		fmt.Println("SeedsFinishedGet" + string(stats.SeedsFinishedGet()))
		fmt.Println("PreprocessorRoutinesGet" + string(stats.PreprocessorRoutinesGet()))
		fmt.Println("ArchiverRoutinesGet" + string(stats.ArchiverRoutinesGet()))
		fmt.Println("PostprocessorRoutinesGet" + string(stats.PostprocessorRoutinesGet()))
	}
}
