package archiver

import (
	"path"

	"github.com/internetarchive/Zeno/internal/pkg/archiver/discard"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	warc "github.com/internetarchive/gowarc"
)

func startWARCWriter() error {
	// Configure WARC rotator settings
	rotatorSettings := warc.NewRotatorSettings()
	rotatorSettings.Prefix = config.Get().WARCPrefix
	rotatorSettings.WARCWriterPoolSize = config.Get().WARCPoolSize
	rotatorSettings.WARCSize = float64(config.Get().WARCSize)
	rotatorSettings.OutputDirectory = path.Join(config.Get().JobPath, "warcs")

	version := utils.GetVersion()
	rotatorSettings.WarcinfoContent.Set("software", "Zeno/"+version.Version+" warc/"+version.WarcVersion)
	if config.Get().WARCOperator != "" {
		rotatorSettings.WarcinfoContent.Set("operator", config.Get().WARCOperator)
	}
	if config.Get().Headless {
		rotatorSettings.WarcinfoContent.Set("zeno-headless", "true")
	}
	// Configure WARC dedupe settings
	dedupeOptions := warc.DedupeOptions{LocalDedupe: !config.Get().DisableLocalDedupe, SizeThreshold: config.Get().WARCDedupeSize, DedupeCacheSize: config.Get().WARCDedupeCacheSize}
	if config.Get().CDXDedupeServer != "" {
		dedupeOptions.CDXDedupe = true
		dedupeOptions.CDXURL = config.Get().CDXDedupeServer
		dedupeOptions.CDXCookie = config.Get().CDXCookie
	}

	if config.Get().DoppelgangerDedupeServer != "" {
		dedupeOptions.DoppelgangerDedupe = true
		dedupeOptions.DoppelgangerHost = config.Get().DoppelgangerDedupeServer
	}

	// Configure WARC discard hook
	discardBuilder := discard.NewBuilder()
	discardBuilder.AddDefaultHooks()
	discardHooksChain := discardBuilder.Build()

	// Configure WARC settings
	WARCSettings := warc.HTTPClientSettings{
		RotatorSettings:  rotatorSettings,
		DedupeOptions:    dedupeOptions,
		DecompressBody:   true,
		DiscardHook:      discardHooksChain,
		VerifyCerts:      config.Get().CertValidation,
		TempDir:          config.Get().WARCTempDir,
		FullOnDisk:       config.Get().WARCOnDisk,
		RandomLocalIP:    config.Get().RandomLocalIP,
		DisableIPv4:      config.Get().DisableIPv4,
		DisableIPv6:      config.Get().DisableIPv6,
		IPv6AnyIP:        config.Get().IPv6AnyIP,
		ConnReadDeadline: config.Get().ConnReadDeadline,
		DigestAlgorithm:  warc.GetDigestFromPrefix(config.Get().WARCDigestAlgorithm),
		Proxy:            config.Get().Proxy,
	}

	// Instantiate WARC client
	var err error

	// Instantiate WARC client
	globalArchiver.Client, err = warc.NewWARCWritingHTTPClient(WARCSettings)
	if err != nil {
		logger.Error("unable to init WARC HTTP client", "err", err.Error(), "func", "archiver.startWARCWriter")
		return err
	}

	go func() {
		for err := range globalArchiver.Client.ErrChan {
			logger.Error("WARC writer error", "err", err.Err.Error(), "func", err.Func)
		}
	}()

	// Set the timeouts
	if config.Get().HTTPTimeout > 0 {
		globalArchiver.Client.Timeout = config.Get().HTTPTimeout
	}

	return nil
}

type WARCStats struct {
	WARCWritingQueueSize         int64
	WARCTotalBytesArchived       int64
	CDXDedupeTotalBytes          int64
	DoppelgangerDedupeTotalBytes int64
	LocalDedupeTotalBytes        int64
	CDXDedupeTotal               int64
	DoppelgangerDedupeTotal      int64
	LocalDedupeTotal             int64
}

func GetStats() WARCStats {
	var stats WARCStats

	c := globalArchiver.Client
	stats.WARCWritingQueueSize = int64(c.WaitGroup.Size())
	stats.WARCTotalBytesArchived = c.DataTotal.Load()
	stats.CDXDedupeTotalBytes = c.CDXDedupeTotalBytes.Load()
	stats.DoppelgangerDedupeTotalBytes = c.DoppelgangerDedupeTotalBytes.Load()
	stats.LocalDedupeTotalBytes = c.LocalDedupeTotalBytes.Load()
	stats.CDXDedupeTotal = c.CDXDedupeTotal.Load()
	stats.DoppelgangerDedupeTotal = c.DoppelgangerDedupeTotal.Load()
	stats.LocalDedupeTotal = c.LocalDedupeTotal.Load()
	return stats
}

// This function is used in multiple places so it can't be replaced by GetStats()
func GetWARCWritingQueueSize() (total int) {
	return globalArchiver.Client.WaitGroup.Size()
}
