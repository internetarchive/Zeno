package controler

import "github.com/internetarchive/Zeno/pkg/models"

var (
	stageChannels []chan *models.Item
)

func makeStageChannel(bufferSize ...int) chan *models.Item {
	var parsedSize int

	if len(bufferSize) == 0 {
		parsedSize = 0
	} else if len(bufferSize) == 1 {
		parsedSize = bufferSize[0]
	} else {
		panic("makeStageChannel: too many arguments, variadic argument should be omitted or a single integer")
	}

	ch := make(chan *models.Item, parsedSize)
	stageChannels = append(stageChannels, ch)
	return ch
}

func closeStageChannels() {
	for _, ch := range stageChannels {
		close(ch)
	}
}
