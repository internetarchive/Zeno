package lq

import (
	"context"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/log"
)

var VacuumInterval = 10 * time.Minute

func (s *LQ) vacuumer() {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "lq.vacuumer",
	})

	// Create a context to manage goroutines
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	ticker := time.NewTicker(VacuumInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Debug("context cancelled, exiting")
			return
		case <-ticker.C:
			logger.Info("vacuuming")
			_, err := s.client.dbWrite.Exec("VACUUM;")
			if err != nil {
				logger.Error("vacuuming failed", err)
			}
			logger.Info("vacuuming complete")
		}
	}

}
