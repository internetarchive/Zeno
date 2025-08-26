package log

import (
	"sync"
	"testing"
)

func TestLoggerRaceCondition(t *testing.T) {
	// ensure logger is stopped before starting
	Stop()

	if err := Start(); err != nil {
		t.Fatalf("Failed to start logger: %v", err)
	}

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() { Debug("message") })
		wg.Go(func() { Info("message") })
		wg.Go(func() { Warn("message") })
		wg.Go(func() { Error("message") })
	}

	stopped := make(chan struct{})
	go func() {
		Stop()
		close(stopped)
	}()

	wg.Wait()

	<-stopped

	loggerMu.RLock()
	defer loggerMu.RUnlock()
	if multiLogger != nil {
		t.Error("Logger should be nil after Stop()")
	}
}

func TestLoggerNilSafety(t *testing.T) {
	Stop()

	Debug("Should not panic when logger is nil")
	Info("Should not panic when logger is nil")
	Warn("Should not panic when logger is nil")
	Error("Should not panic when logger is nil")

	if err := Start(); err != nil {
		t.Fatalf("Failed to start logger: %v", err)
	}

	Debug("Should log when logger is initialized")
	Info("Should log when logger is initialized")
	Warn("Should log when logger is initialized")
	Error("Should log when logger is initialized")

	Stop()

	loggerMu.RLock()
	defer loggerMu.RUnlock()
	if multiLogger != nil {
		t.Error("Logger should be nil after Stop()")
	}
}
