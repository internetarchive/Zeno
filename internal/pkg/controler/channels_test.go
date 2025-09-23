package controler

import (
	"testing"

	"github.com/internetarchive/Zeno/pkg/models"
)

func TestMakeStageChannel(t *testing.T) {
	// Reset state
	stageChannels = nil
	namedChannels = nil

	// Test basic channel creation
	ch := makeStageChannel(10)
	if cap(ch) != 10 {
		t.Errorf("Expected channel capacity 10, got %d", cap(ch))
	}

	// Test named channel creation
	namedCh := makeNamedStageChannel("test_channel", 5)
	if cap(namedCh) != 5 {
		t.Errorf("Expected named channel capacity 5, got %d", cap(namedCh))
	}

	// Test that named channel is tracked
	queueSizes := GetChannelQueueSizes()
	if len(queueSizes) != 1 {
		t.Errorf("Expected 1 named channel, got %d", len(queueSizes))
	}

	if size, exists := queueSizes["test_channel"]; !exists {
		t.Error("Expected 'test_channel' to exist in queue sizes")
	} else if size != 0 {
		t.Errorf("Expected queue size 0, got %d", size)
	}
}

func TestGetChannelQueueSizes(t *testing.T) {
	// Reset state
	stageChannels = nil
	namedChannels = nil

	// Create multiple named channels
	ch1 := makeNamedStageChannel("channel1", 10)
	makeNamedStageChannel("channel2", 5)

	// Add some items to test queue size tracking
	item := &models.Item{}

	select {
	case ch1 <- item:
		// Successfully added item to ch1
	default:
		t.Fatal("Failed to add item to ch1")
	}

	// Get queue sizes
	queueSizes := GetChannelQueueSizes()

	// Verify tracking
	if len(queueSizes) != 2 {
		t.Errorf("Expected 2 channels, got %d", len(queueSizes))
	}

	if size := queueSizes["channel1"]; size != 1 {
		t.Errorf("Expected channel1 size 1, got %d", size)
	}

	if size := queueSizes["channel2"]; size != 0 {
		t.Errorf("Expected channel2 size 0, got %d", size)
	}
}
