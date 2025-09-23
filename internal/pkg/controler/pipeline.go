package controler

import (
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

var sourceInterface source.Source
var hqOutlinks *hq.HQ

/**
 * Channel description:
 * reactorOutputChan: reactor → preprocessor
 * preprocessorOutputChan: preprocessor → archiver
 * archiverOutputChan: archiver → postprocessor
 * postprocessorOutputChan: postprocessor → finisher
 * finisherFinishChan: finisher → HQ or LQ. Notify when a seed is finished.
 * finisherProduceChan: HQ or LQ → finisher. Send fresh seeds.
 */
func startPipeline() error {
	err := log.Start()
	if err != nil {
		fmt.Println("error starting logger", "err", err.Error())
		return err
	}

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "controler.StartPipeline",
	})
	if err := os.MkdirAll(config.Get().JobPath, 0755); err != nil {
		logger.Error("can't create job directory", "err", err.Error())
		return err
	}
	if err := watchers.CheckDiskUsage(config.Get().JobPath); err != nil {
		logger.Error("can't start Zeno", "err", err.Error())
		return err
	}

	err = stats.Init()
	if err != nil {
		logger.Error("error initializing stats", "err", err.Error())
		return err
	}

	// Set the channel queue size getter for stats monitoring
	stats.SetChannelQueueSizeGetter(GetChannelQueueSizes)

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
			return err
		}
	}

	// Start the reactor that will receive
	reactorOutputChan := makeNamedStageChannel("reactor_to_preprocessor", config.Get().WorkersCount)
	err = reactor.Start(config.Get().WorkersCount, reactorOutputChan)
	if err != nil {
		logger.Error("error starting reactor", "err", err.Error())
		return err
	}

	// If needed, create the seencheck DB (only if not using HQ)
	if config.Get().UseSeencheck && !config.Get().UseHQ {
		err := seencheck.Start(config.Get().JobPath)
		if err != nil {
			logger.Error("unable to start seencheck", "err", err.Error())
			return err
		}
	}

	preprocessorOutputChan := makeNamedStageChannel("preprocessor_to_archiver", config.Get().WorkersCount)
	err = preprocessor.Start(reactorOutputChan, preprocessorOutputChan)
	if err != nil {
		logger.Error("error starting preprocessor", "err", err.Error())
		return err
	}

	archiverOutputChan := makeNamedStageChannel("archiver_to_postprocessor", config.Get().WorkersCount)
	err = archiver.Start(preprocessorOutputChan, archiverOutputChan)
	if err != nil {
		logger.Error("error starting archiver", "err", err.Error())
		return err
	}

	// Used by optional 2nd HQ instance just to gather outlinks to a different project
	hqOutlinksFinishChan := makeNamedStageChannel("hq_outlinks_finish", config.Get().WorkersCount)
	hqOutlinksProduceChan := makeNamedStageChannel("hq_outlinks_produce", config.Get().WorkersCount)

	// Start the WARC writing queue watcher
	watchers.StartWatchWARCWritingQueue(1*time.Second, 2*time.Second, 250*time.Millisecond)

	postprocessorOutputChan := makeNamedStageChannel("postprocessor_to_finisher", config.Get().WorkersCount)
	err = postprocessor.Start(archiverOutputChan, postprocessorOutputChan, hqOutlinksProduceChan)
	if err != nil {
		logger.Error("error starting postprocessor", "err", err.Error())
		return err
	}

	finisherFinishChan := makeNamedStageChannel("finisher_to_source", config.Get().WorkersCount)
	finisherProduceChan := makeNamedStageChannel("source_to_finisher", config.Get().WorkersCount)

	if config.Get().UseHQ {
		hqSource := hq.New(config.Get().HQKey, config.Get().HQSecret, config.Get().HQProject, config.Get().HQAddress)
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
		return err
	}

	// Optional 2nd HQ instance just to gather outlinks to a different project
	if config.Get().UseHQ && config.Get().HQOutlinksProject != "" && config.Get().HQOutlinksHopLimit > 0 {
		hqOutlinks = hq.New(config.Get().HQKey, config.Get().HQSecret, config.Get().HQOutlinksProject, config.Get().HQAddress)
		hqOutlinks.Start(hqOutlinksFinishChan, hqOutlinksProduceChan)
	}

	err = finisher.Start(postprocessorOutputChan, finisherFinishChan, finisherProduceChan)
	if err != nil {
		logger.Error("error starting finisher", "err", err.Error())
		return err
	}

	// Pipe in the reactor the input seeds if any
	if len(config.Get().InputSeeds) > 0 {
		for _, seed := range config.Get().InputSeeds {
			parsedURL, err := models.NewURL(seed)
			if err != nil {
				return err
			}

			item := models.NewItem(&parsedURL, "")
			item.SetSource(models.ItemSourceQueue)

			err = reactor.ReceiveInsert(item)
			if err != nil {
				logger.Error("unable to insert seed", "err", err.Error())
				return err
			}
		}
	}
	return nil
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

	sourceInterface.Stop()

	if config.Get().UseHQ && config.Get().HQOutlinksProject != "" && config.Get().HQOutlinksHopLimit > 0 {
		hqOutlinks.Stop()
	}

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

	if config.Get().ConsulRegister {
		consul.Stop()
	}

	logger.Info("done, logs are flushing and will be closed")

	log.Stop()
}
