package controler

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/internetarchive/Zeno/internal/pkg/log"
)

var signalWatcherCtx, signalWatcherCancel = context.WithCancel(context.Background())

// WatchSignals listens for OS signals and handles them gracefully
func WatchSignals() {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "controler.signalWatcher",
	})
	// Handle OS signals for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-signalWatcherCtx.Done():
		return
	case <-signalChan:
		logger.Info("received shutdown signal, stopping services...")
		// Catch a second signal to force exit
		go func() {
			<-signalChan
			logger.Info("received second shutdown signal, forcing exit...")
			os.Exit(1)
		}()

		Stop()
	}
}
