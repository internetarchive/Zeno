package reactor

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gocrawlhq"
)

func TestReactorE2E(t *testing.T) {
	// Initialize the reactor with a maximum of 5 tokens
	outputChan := make(chan *models.Seed)
	err := Start(5, outputChan)
	if err != nil {
		t.Logf("Error starting reactor: %s", err)
		return
	}
	defer Stop()

	// Consume items from the output channel, start 5 goroutines
	for i := 0; i < 5; i++ {
		go func(t *testing.T) {
			for {
				select {
				case item := <-outputChan:
					// Send feedback for the consumed item
					if item.Source != models.SeedSourceFeedback {
						err := ReceiveFeedback(item)
						if err != nil {
							t.Fatalf("Error sending feedback: %s - %s", err, item.UUID.String())
						}
						continue
					}

					// Mark the item as finished
					if item.Source == models.SeedSourceFeedback {
						err := MarkAsFinished(item)
						if err != nil {
							t.Fatalf("Error marking item as finished: %s", err)
						}
						continue
					}
				}
			}
		}(t)
	}

	// Create mock seeds
	mockSeeds := []*models.Seed{}
	for i := 0; i <= 1000; i++ {
		uuid := uuid.New()
		mockSeeds = append(mockSeeds, &models.Seed{
			UUID:   &uuid,
			URL:    &gocrawlhq.URL{Value: fmt.Sprintf("http://example.com/%d", i)},
			Status: models.SeedFresh,
			Source: models.SeedSourceHQ,
		})
	}

	// Queue mock seeds to the source channel
	for _, seed := range mockSeeds {
		err := ReceiveSource(seed)
		if err != nil {
			t.Fatalf("Error queuing seed to source channel: %s", err)
		}
	}

	// Allow some time for processing
	time.Sleep(5 * time.Second)
	if len(GetStateTable()) > 0 {
		t.Fatalf("State table is not empty: %s", GetStateTable())
	}
}
