package controler

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/internal/pkg/api"
	"github.com/internetarchive/Zeno/internal/pkg/archiver"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/consul"
	"github.com/internetarchive/Zeno/internal/pkg/controler/watchers"
	"github.com/internetarchive/Zeno/internal/pkg/finisher"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/postprocessor"
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor"
	"github.com/internetarchive/Zeno/internal/pkg/preprocessor/seencheck"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
	"github.com/internetarchive/Zeno/internal/pkg/source/hq"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/internetarchive/Zeno/pkg/models"
)

func startPipeline() {
	if err := os.MkdirAll(config.Get().JobPath, 0755); err != nil {
		fmt.Printf("can't create job directory: %s\n", err)
		os.Exit(1)
	}

	if err := watchers.CheckDiskUsage(config.Get().JobPath); err != nil {
		fmt.Printf("can't start Zeno: %s\n", err)
		os.Exit(1)
	}

	err := log.Start()
	if err != nil {
		fmt.Println("error starting logger", "err", err.Error())
		panic(err)
	}

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "controler.StartPipeline",
	})

	err = stats.Init()
	if err != nil {
		logger.Error("error initializing stats", "err", err.Error())
		panic(err)
	}

	// Start the disk watcher
	go watchers.WatchDiskSpace(config.Get().JobPath, 5*time.Second)

	// Start the API server if needed
	if config.Get().API {
		api.Start()
	}

	// Register Zeno as Consul service if needed
	if config.Get().ConsulRegister {
		err := consul.Register()
		if err != nil {
			logger.Error("error registering Zeno in Consul", "err", err.Error())
			panic(err)
		}
	}

	// Start the reactor that will receive
	reactorOutputChan := makeStageChannel(config.Get().WorkersCount)
	err = reactor.Start(config.Get().WorkersCount, reactorOutputChan)
	if err != nil {
		logger.Error("error starting reactor", "err", err.Error())
		panic(err)
	}

	// If needed, create the seencheck DB (only if not using HQ)
	if config.Get().UseSeencheck && !config.Get().UseHQ {
		err := seencheck.Start(config.Get().JobPath)
		if err != nil {
			logger.Error("unable to start seencheck", "err", err.Error())
			panic(err)
		}
	}

	preprocessorOutputChan := makeStageChannel(config.Get().WorkersCount)
	err = preprocessor.Start(reactorOutputChan, preprocessorOutputChan)
	if err != nil {
		logger.Error("error starting preprocessor", "err", err.Error())
		panic(err)
	}

	archiverOutputChan := makeStageChannel(config.Get().WorkersCount)
	err = archiver.Start(preprocessorOutputChan, archiverOutputChan)
	if err != nil {
		logger.Error("error starting archiver", "err", err.Error())
		panic(err)
	}

	// Start the WARC writing queue watcher
	go watchers.WatchWARCWritingQueue(5 * time.Second)

	postprocessorOutputChan := makeStageChannel(config.Get().WorkersCount)
	err = postprocessor.Start(archiverOutputChan, postprocessorOutputChan)
	if err != nil {
		logger.Error("error starting postprocessor", "err", err.Error())
		panic(err)
	}

	finisherFinishChan := makeStageChannel(config.Get().WorkersCount)
	finisherProduceChan := makeStageChannel(config.Get().WorkersCount)

	if config.Get().UseHQ {
		logger.Info("starting hq")
		err = hq.Start(finisherFinishChan, finisherProduceChan)
		if err != nil {
			logger.Error("error starting hq source, retrying", "err", err.Error())
			panic(err)
		}
	} else {
		// Means we're using the to-be-implemented local queue, for the moment we're just gonna consume the channels
		go func() {
			for {
				select {
				case _, ok := <-finisherFinishChan:
					if !ok {
						return
					}
				case _, ok := <-finisherProduceChan:
					if !ok {
						return
					}
				}
			}
		}()
	}

	err = finisher.Start(postprocessorOutputChan, finisherFinishChan, finisherProduceChan)
	if err != nil {
		logger.Error("error starting finisher", "err", err.Error())
		panic(err)
	}

	// Pipe in the reactor the input seeds if any
	if len(config.Get().InputSeeds) > 0 {
		for _, seed := range config.Get().InputSeeds {
			parsedURL := &models.URL{Raw: seed}
			err := parsedURL.Parse()
			if err != nil {
				panic(err)
			}

			item := models.NewItem(uuid.New().String(), parsedURL, "")
			item.SetSource(models.ItemSourceQueue)

			err = reactor.ReceiveInsert(item)
			if err != nil {
				logger.Error("unable to insert seed", "err", err.Error())
				panic(err)
			}
		}
	}
}

func stopPipeline() {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "controler.stopPipeline",
	})

	watchers.StopDiskWatcher()
	watchers.StopWARCWritingQueueWatcher()

	reactor.Freeze()

	preprocessor.Stop()
	archiver.Stop()
	postprocessor.Stop()
	finisher.Stop()

	if config.Get().UseSeencheck && !config.Get().UseHQ {
		seencheck.Close()
	}

	if config.Get().UseHQ {
		hq.Stop()
	}

	reactor.Stop()

	if config.Get().WARCTempDir != "" {
		err := os.Remove(config.Get().WARCTempDir)
		if err != nil {
			logger.Error("unable to remove temp dir", "err", err.Error())
		}
	}

	api.Stop(5 * time.Second)

	if config.Get().ConsulRegister {
		consul.Stop()
	}

	logger.Info("done, logs are flushing and will be closed")

	log.Stop()
}
