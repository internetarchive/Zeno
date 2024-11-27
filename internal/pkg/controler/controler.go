package controler

func Start() {
	startPipeline()
}

func Stop() {
	stopPipeline()
	closeStageChannels()
	if config.Get().WARCTempDir != "" {
		err := os.Remove(config.Get().WARCTempDir)
		if err != nil {
			logger.Error("unable to remove temp dir", "err", err.Error())
		}
	}
}
