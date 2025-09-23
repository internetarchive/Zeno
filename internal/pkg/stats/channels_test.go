package stats

import (
	"testing"
)

func TestChannelQueueSizeGetter(t *testing.T) {
	// Reset getter
	channelQueueSizeGetter = nil

	// Test with no getter set
	sizes := GetChannelQueueSizes()
	if len(sizes) != 0 {
		t.Errorf("Expected empty map when no getter set, got %d items", len(sizes))
	}

	// Set a test getter
	testData := map[string]int{
		"test_channel_1": 5,
		"test_channel_2": 10,
	}

	SetChannelQueueSizeGetter(func() map[string]int {
		return testData
	})

	// Test with getter set
	sizes = GetChannelQueueSizes()
	if len(sizes) != 2 {
		t.Errorf("Expected 2 channels, got %d", len(sizes))
	}

	if sizes["test_channel_1"] != 5 {
		t.Errorf("Expected test_channel_1 size 5, got %d", sizes["test_channel_1"])
	}

	if sizes["test_channel_2"] != 10 {
		t.Errorf("Expected test_channel_2 size 10, got %d", sizes["test_channel_2"])
	}
}
