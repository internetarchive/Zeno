package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/internetarchive/gocrawlhq"
)

func main() {
	// Initialize the reactor with a maximum of 5 tokens
	outputChan := make(chan *models.Seed)
	err := reactor.Start(5, outputChan)
	if err != nil {
		fmt.Println("Error starting reactor:", err)
		return
	}
	defer reactor.Stop()

	// Consume items from the output channel, start 5 goroutines
	for i := 0; i < 5; i++ {
		go func() {
			for {
				select {
				case item := <-outputChan:
					fmt.Println("Consumed item from output channel:", item.URL.Value, item.Source)

					// Send feedback for the consumed item
					if item.Source != models.SeedSourceFeedback {
						err := reactor.ReceiveFeedback(item)
						if err != nil {
							fmt.Println("Error sending feedback:", err)
						}
						continue
					}

					// Mark the item as finished
					if item.Source == models.SeedSourceFeedback {
						err := reactor.MarkAsFinished(item)
						if err != nil {
							fmt.Println("Error marking item as finished:", err)
						}
						fmt.Println("Marked item as finished:", item.URL.Value)
						continue
					}
				}
			}
		}()
	}

	// Create mock seeds
	uuid1 := uuid.New()
	uuid2 := uuid.New()
	mockSeeds := []*models.Seed{
		{
			UUID:   &uuid1,
			URL:    &gocrawlhq.URL{Value: "http://example.com/1"},
			Status: models.SeedFresh,
			Source: models.SeedSourceQueue,
		},
		{
			UUID:   &uuid2,
			URL:    &gocrawlhq.URL{Value: "http://example.com/2"},
			Status: models.SeedFresh,
			Source: models.SeedSourceQueue,
		},
	}

	// Queue mock seeds to the source channel
	for _, seed := range mockSeeds {
		err := reactor.ReceiveSource(seed)
		if err != nil {
			fmt.Println("Error queuing seed to source channel:", err)
		}
	}

	// Allow some time for processing
	time.Sleep(10 * time.Second)
}
