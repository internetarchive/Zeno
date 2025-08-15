package controler

import (
	"context"
	"fmt"
	"os"
	"time"

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
	"github.com/internetarchive/Zeno/internal/pkg/source"
	"github.com/internetarchive/Zeno/internal/pkg/source/hq"
	"github.com/internetarchive/Zeno/internal/pkg/source/lq"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/internetarchive/Zeno/pkg/models"
)

var (
	rootContext       context.Context
	cancelRootContext context.CancelFunc
	sourceInterface   source.Source
)

/**
 * Channel description:
 * reactorOutputChan: reactor → preprocessor
 * preprocessorOutputChan: preprocessor → archiver
 * archiverOutputChan: archiver → postprocessor
 * postprocessorOutputChan: postprocessor → finisher
 * finisherFinishChan: finisher → HQ or LQ. Notify when a seed is finished.
 * finisherProduceChan: HQ or LQ → finisher. Send fresh seeds.
 */
func startPipeline() {
	if err := os.MkdirAll(config.Get().JobPath, 0755); err != nil {
		fmt.Printf("can't create job directory: %s\n", err)
		os.Exit(1)
	}

	rootContext, cancelRootContext = context.WithCancel(context.Background())

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
		err := consul.Register(rootContext)
		if err != nil {
			logger.Error("error registering Zeno in Consul", "err", err.Error())
			panic(err)
		}
	}

	// Start the reactor that will receive
	reactorOutputChan := makeStageChannel(config.Get().WorkersCount)
	err = reactor.Start(rootContext, config.Get().WorkersCount, reactorOutputChan)
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
	err = preprocessor.Start(rootContext, reactorOutputChan, preprocessorOutputChan)
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
	watchers.StartWatchWARCWritingQueue(rootContext, 1*time.Second, 2*time.Second, 250*time.Millisecond)

	postprocessorOutputChan := makeStageChannel(config.Get().WorkersCount)
	err = postprocessor.Start(rootContext, archiverOutputChan, postprocessorOutputChan)
	if err != nil {
		logger.Error("error starting postprocessor", "err", err.Error())
		panic(err)
	}

	finisherFinishChan := makeStageChannel(config.Get().WorkersCount)
	finisherProduceChan := makeStageChannel(config.Get().WorkersCount)

	if config.Get().UseHQ {
		hqSource := hq.New()
		preprocessor.SetSeenchecker(hqSource.SeencheckItem)
		sourceInterface = hqSource
	} else {
		lqSource := lq.New()
		if config.Get().UseSeencheck {
			preprocessor.SetSeenchecker(seencheck.SeencheckItem)
		}
		sourceInterface = lqSource
	}

	logger.Info("starting source", "source", sourceInterface.Name())
	err = sourceInterface.Start(finisherFinishChan, finisherProduceChan)
	if err != nil {
		logger.Error("error starting source", "source", sourceInterface.Name(), "err", err.Error())
		panic(err)
	}

	err = finisher.Start(rootContext, postprocessorOutputChan, finisherFinishChan, finisherProduceChan)
	if err != nil {
		logger.Error("error starting finisher", "err", err.Error())
		panic(err)
	}

	// Pipe in the reactor the input seeds if any
	if len(config.Get().InputSeeds) > 0 {
		for _, seed := range config.Get().InputSeeds {
			parsedURL, err := models.NewURL(seed)
			if err != nil {
				panic(err)
			}

			item := models.NewItem(&parsedURL, "")
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

	if cancelRootContext != nil {
		cancelRootContext()
	}

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

	sourceInterface.Stop()

	reactor.Stop()

	if config.Get().WARCTempDir != "" {
		err := os.Remove(config.Get().WARCTempDir)
		if err != nil {
			logger.Error("unable to remove temp dir", "err", err.Error())
		}
	}

	if config.Get().API {
		api.Stop(5 * time.Second)
	}

	logger.Info("done, logs are flushing and will be closed")

	log.Stop()
}
