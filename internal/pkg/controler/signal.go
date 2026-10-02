package controler

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/internetarchive/Zeno/v2/internal/pkg/log"
)

var signalWatcherCtx, signalWatcherCancel = context.WithCancel(context.Background())
var SignalChan = make(chan os.Signal, 1)

// create an internal channel to allow functions inside controler to start a shutdown
var stopRequested = make(chan struct{}, 1)

// requestStop starts a shutdown in the same way SIGTERM through SignalChan does.
func requestStop() {
	select {
	case stopRequested <- struct{}{}:
	default:
	}
}

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
	case <-stopRequested:
		logger.Info("shutdown requested, stopping services...")
	}

	// Catch a second signal to force exit
	go func() {
		<-SignalChan
		logger.Info("received second shutdown signal, forcing exit...")
		os.Exit(1)
	}()

	Stop()
}
