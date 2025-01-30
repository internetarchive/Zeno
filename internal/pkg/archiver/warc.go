package archiver

import (
	"context"
	"os"
	"path"
	"time"

	"github.com/CorentinB/warc"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
)

func startWARCWriter() {
	// Configure WARC rotator settings
	rotatorSettings := warc.NewRotatorSettings()
	rotatorSettings.Prefix = config.Get().WARCPrefix
	rotatorSettings.WARCWriterPoolSize = config.Get().WARCPoolSize
	rotatorSettings.WarcSize = float64(config.Get().WARCSize)
	rotatorSettings.OutputDirectory = path.Join(config.Get().JobPath, "warcs")

	// Configure WARC dedupe settings
	dedupeOptions := warc.DedupeOptions{LocalDedupe: !config.Get().DisableLocalDedupe, SizeThreshold: config.Get().WARCDedupeSize}
	if config.Get().CDXDedupeServer != "" {
		dedupeOptions = warc.DedupeOptions{
			LocalDedupe:   !config.Get().DisableLocalDedupe,
			CDXDedupe:     true,
			CDXURL:        config.Get().CDXDedupeServer,
			CDXCookie:     config.Get().CDXCookie,
			SizeThreshold: config.Get().WARCDedupeSize,
		}
	}

	// Configure WARC settings
	WARCSettings := warc.HTTPClientSettings{
		RotatorSettings:     rotatorSettings,
		DedupeOptions:       dedupeOptions,
		DecompressBody:      true,
		SkipHTTPStatusCodes: []int{429},
		VerifyCerts:         config.Get().CertValidation,
		TempDir:             config.Get().WARCTempDir,
		FullOnDisk:          config.Get().WARCOnDisk,
		RandomLocalIP:       config.Get().RandomLocalIP,
		DisableIPv4:         config.Get().DisableIPv4,
		DisableIPv6:         config.Get().DisableIPv6,
		IPv6AnyIP:           config.Get().IPv6AnyIP,
	}

	// Instantiate WARC client
	var err error
	if config.Get().Proxy != "" {
		proxiedWARCSettings := WARCSettings
		proxiedWARCSettings.Proxy = config.Get().Proxy
		globalArchiver.ClientWithProxy, err = warc.NewWARCWritingHTTPClient(proxiedWARCSettings)
		if err != nil {
			logger.Error("unable to init proxied WARC HTTP client", "err", err.Error(), "func", "archiver.startWARCWriter")
			os.Exit(1)
		}

		go func() {
			for {
				select {
				case <-globalArchiver.ctx.Done():
					return
				case err := <-globalArchiver.ClientWithProxy.ErrChan:
					logger.Error("WithProxy WARC writer error", "err", err.Err.Error(), "func", err.Func)
				}
			}
		}()
	}

	// Even if a proxied client has been set, we want to create an non-proxied one
	// if DomainsBypassProxy is used. The domains specified in this slice won't go
	// through the proxied client, but through a "normal" client
	if config.Get().Proxy == "" || len(config.Get().DomainsBypassProxy) > 0 {
		globalArchiver.Client, err = warc.NewWARCWritingHTTPClient(WARCSettings)
		if err != nil {
			logger.Error("unable to init WARC HTTP client", "err", err.Error(), "func", "archiver.startWARCWriter")
			os.Exit(1)
		}

		go func() {
			for {
				select {
				case <-globalArchiver.ctx.Done():
					return
				case err := <-globalArchiver.Client.ErrChan:
					logger.Error("WARC writer error", "err", err.Err.Error(), "func", err.Func)
				}
			}
		}()
	}

	// Set the timeouts
	if config.Get().HTTPTimeout > 0 {
		if globalArchiver.Client != nil {
			globalArchiver.Client.Timeout = time.Duration(config.Get().HTTPTimeout) * time.Second
		}

		if globalArchiver.ClientWithProxy != nil {
			globalArchiver.ClientWithProxy.Timeout = time.Duration(config.Get().HTTPTimeout) * time.Second
		}
	}
}

// GetWARCWritingQueueSize returns the total number of items in the WARC writing queue
func GetWARCWritingQueueSize() (total int) {
	for _, c := range []*warc.CustomHTTPClient{globalArchiver.Client, globalArchiver.ClientWithProxy} {
		if c != nil {
			total += c.WaitGroup.Size()
		}
	}

	return total
}

var (
	watchWARCWritingQueueContext, watchWARCWritingQueueCancel = context.WithCancel(context.Background())
)

func watchWARCWritingQueue(interval time.Duration) {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "archiver.warcWritingQueueWatcher",
	})

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-watchWARCWritingQueueContext.Done():
			logger.Debug("closed")
			return
		case <-ticker.C:
			stats.WarcWritingQueueSizeSet(int64(GetWARCWritingQueueSize()))
		}
	}
}
