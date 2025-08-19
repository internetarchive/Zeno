package controler

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/internetarchive/Zeno/internal/pkg/log"
)

var signalWatcherCtx, signalWatcherCancel = context.WithCancel(context.Background())
var SignalChan = make(chan os.Signal, 1)

// WatchSignals listens for OS signals and handles them gracefully
func WatchSignals() {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "controler.signalWatcher",
	})
	// Handle OS signals for graceful shutdown
	signal.Notify(SignalChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-signalWatcherCtx.Done():
		return
	case <-SignalChan:
		logger.Info("received shutdown signal, stopping services...")
		// Catch a second signal to force exit
		go func() {
			<-SignalChan
			logger.Info("received second shutdown signal, forcing exit...")
			os.Exit(1)
		}()

		Stop()
	}
}
