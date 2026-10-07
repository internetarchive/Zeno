// Package controler provides a way to start and stop the pipeline.
package controler

import "sync"

// Start initializes the pipeline.
func Start() {
	if err := startPipeline(); err != nil {
		panic(err)
	}
}

var stopOnce sync.Once

// Stop stops the pipeline. Only the first call has any effect; later calls
// are no-ops.
func Stop() {
	stopOnce.Do(func() {
		stopPipeline()
		closeStageChannels()
	})
}
