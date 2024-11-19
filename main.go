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

	"github.com/internetarchive/Zeno/cmd"
	"github.com/internetarchive/Zeno/internal/pkg/archiver"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/finisher"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor"
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
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
	err := reactor.Start(config.Get().WorkersCount, reactorOutputChan)
	if err != nil {
		logger.Error("error starting reactor", "err", err.Error())
		return
	}
	defer reactor.Stop()

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
}
