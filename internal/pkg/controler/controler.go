package controler

func Start() {
	startPipeline()
}

func Stop() {
	stopPipeline()
	closeStageChannels()
}
