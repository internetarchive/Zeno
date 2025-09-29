package stats

// ChannelQueueSizeGetter is a function type that returns channel queue sizes
type ChannelQueueSizeGetter func() map[string]int

var channelQueueSizeGetter ChannelQueueSizeGetter

// SetChannelQueueSizeGetter sets the function to get channel queue sizes
func SetChannelQueueSizeGetter(getter ChannelQueueSizeGetter) {
	channelQueueSizeGetter = getter
}

// GetChannelQueueSizes returns the current channel queue sizes
func GetChannelQueueSizes() map[string]int {
	if channelQueueSizeGetter != nil {
		return channelQueueSizeGetter()
	}
	return make(map[string]int)
}
