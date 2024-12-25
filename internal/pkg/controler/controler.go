// Package controler provides a way to start and stop the pipeline.
package controler

// Start initializes the pipeline.
func Start() {
	startPipeline()
}

// Stop stops the pipeline.
func Stop() {
	stopPipeline()
	closeStageChannels()
}
