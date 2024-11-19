package reactor

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/pkg/models"
)

func TestReactor_E2E_Balanced(t *testing.T) {
	_testerFunc(50, 50, 1000, t)
}

func TestReactor_E2E_Unbalanced_MoreConsumers(t *testing.T) {
	_testerFunc(10, 50, 1000, t)
}

func TestReactor_E2E_Unbalanced_MoreTokens(t *testing.T) {
	_testerFunc(50, 10, 1000, t)
}

func TestReactor_E2E_BalancedBig(t *testing.T) {
	_testerFunc(5000, 5000, 1000000, t)
}

func TestReactor_E2E_UnbalancedBig_MoreConsumers(t *testing.T) {
	_testerFunc(50, 5000, 1000000, t)
}

func TestReactor_E2E_UnbalancedBig_MoreTokens(t *testing.T) {
	_testerFunc(50, 5000, 1000000, t)
}

func _testerFunc(tokens, consumers, seeds int, t testing.TB) {
	// Initialize the reactor with a maximum of 5 tokens
	outputChan := make(chan *models.Item)
	err := Start(1, outputChan)

	if err != nil {
		t.Logf("Error starting reactor: %s", err)
		return
	}
	defer log.Shutdown()
	defer Stop()

	// Channel to collect errors from goroutines
	errorChan := make(chan error)

	// Consume items from the output channel, start 5 goroutines
	for i := 0; i < 5; i++ {
		go func() {
			for {
				select {
				case item := <-outputChan:
					if item == nil {
						continue
					}

					// Send feedback for the consumed item
					if item.Source != models.ItemSourceFeedback {
						err := ReceiveFeedback(item)
						if err != nil {
							errorChan <- fmt.Errorf("Error sending feedback: %s - %s", err, item.UUID.String())
						}
						continue
					}

					// Mark the item as finished
					if item.Source == models.ItemSourceFeedback {
						err := MarkAsFinished(item)
						if err != nil {
							errorChan <- fmt.Errorf("Error marking item as finished: %s", err)
						}
						continue
					}
				}
			}
		}()
	}

	// Handle errors from goroutines
	go func() {
		for err := range errorChan {
			t.Error(err)
		}
	}()

	// Create mock seeds
	mockItems := []*models.Item{}
	for i := 0; i <= 1000; i++ {
		uuid := uuid.New()
		mockItems = append(mockItems, &models.Item{
			UUID:   &uuid,
			URL:    &models.URL{Raw: fmt.Sprintf("http://example.com/%d", i)},
			Status: models.ItemFresh,
			Source: models.ItemSourceHQ,
		})
	}

	// Queue mock seeds to the source channel
	for _, seed := range mockItems {
		err := ReceiveInsert(seed)
		if err != nil {
			t.Fatalf("Error queuing seed to source channel: %s", err)
		}
	}

	// Allow some time for processing
	for {
		select {
		case <-time.After(5 * time.Second):
			if len(GetStateTable()) > 0 {
				t.Fatalf("State table is not empty: %s", GetStateTable())
			}
			t.Fatalf("Timeout waiting for reactor to finish processing")
		default:
			if len(GetStateTable()) == 0 {
				return
			}
		}
	}
}