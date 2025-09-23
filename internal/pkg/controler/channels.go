package controler

import (
	"sync"

	"github.com/internetarchive/Zeno/pkg/models"
)

// NamedChannel holds a channel with its associated name for monitoring
type NamedChannel struct {
	Name    string
	Channel chan *models.Item
}

var (
	stageChannels      []chan *models.Item
	namedChannels      []NamedChannel
	namedChannelsMutex sync.RWMutex
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

// makeNamedStageChannel creates a channel with a name for monitoring purposes
func makeNamedStageChannel(name string, bufferSize ...int) chan *models.Item {
	ch := makeStageChannel(bufferSize...)

	namedChannelsMutex.Lock()
	namedChannels = append(namedChannels, NamedChannel{
		Name:    name,
		Channel: ch,
	})
	namedChannelsMutex.Unlock()

	return ch
}

// GetChannelQueueSizes returns the current queue sizes for all named channels
func GetChannelQueueSizes() map[string]int {
	namedChannelsMutex.RLock()
	defer namedChannelsMutex.RUnlock()

	queueSizes := make(map[string]int)
	for _, namedCh := range namedChannels {
		queueSizes[namedCh.Name] = len(namedCh.Channel)
	}
	return queueSizes
}

func closeStageChannels() {
	for _, ch := range stageChannels {
		close(ch)
	}
}
