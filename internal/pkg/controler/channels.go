package controler

import "github.com/internetarchive/Zeno/pkg/models"

var (
	stageChannels []chan *models.Item
)

func makeStageChannel() chan *models.Item {
	ch := make(chan *models.Item)
	stageChannels = append(stageChannels, ch)
	return ch
}

func closeStageChannels() {
	for _, ch := range stageChannels {
		close(ch)
	}
}
