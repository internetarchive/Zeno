package reactor

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	_testerFunc(5000, 5000, 100000, t)
}

func TestReactor_E2E_UnbalancedBig_MoreConsumers(t *testing.T) {
	_testerFunc(50, 5000, 100000, t)
}

func TestReactor_E2E_UnbalancedBig_MoreTokens(t *testing.T) {
	_testerFunc(5000, 50, 100000, t)
}

func _testerFunc(tokens, consumers, seeds int, t testing.TB) {
	outputChan := make(chan *models.Item)
	err := Start(tokens, outputChan)

	var consumedCount atomic.Int64
	consumedCount.Store(0)

	if err != nil {
		t.Logf("Error starting reactor: %s", err)
		return
	}

	// Channel to collect errors from goroutines
	fatalChan := make(chan error, consumers)

	// WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Context to cancel consumers
	ctx, cancel := context.WithCancel(context.Background())

	// Consume items from the output channel, start 5 goroutines
	for i := 0; i < consumers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case item := <-outputChan:
					if item == nil {
						continue
					}

					// Send feedback for the consumed item
					if item.GetSource() != models.ItemSourceFeedback {
						err := ReceiveFeedback(item)
						if err != nil {
							fatalChan <- fmt.Errorf("Error sending feedback: %s - %s", err, item.GetID())
						}
						continue
					}

					// Mark the item as finished
					if item.GetSource() == models.ItemSourceFeedback {
						err := MarkAsFinished(item)
						if err != nil {
							fatalChan <- fmt.Errorf("Error marking item as finished: %s", err)
						}
						consumedCount.Add(1)
						continue
					}
				}
			}
		}()
	}

	// Create mock seeds
	mockItems := []*models.Item{}
	for i := 0; i < seeds; i++ {
		newItem := models.NewItem(&models.URL{Raw: fmt.Sprintf("http://example.com/%d", i)}, "")
		newItem.SetSource(models.ItemSourceInsert)
		newItem.SetStatus(models.ItemFresh)
		mockItems = append(mockItems, newItem)
	}

	// Queue mock seeds to the source channel
	for _, seed := range mockItems {
		err := ReceiveInsert(seed)
		if err != nil {
			cancel()
			wg.Wait()
			Stop()
			log.Stop()
			t.Errorf("Error queuing seed to source channel: %s", err)
			return
		}
	}

	// Allow some time for processing
	for {
		select {
		case err := <-fatalChan:
			cancel()
			wg.Wait()
			Stop()
			log.Stop()
			t.Errorf("Received error while processing %s", err)
			return
		case <-time.After(5 * time.Second):
			cancel()
			wg.Wait()
			if len(GetStateTable()) > 0 {
				t.Errorf("State table is not empty: %s", GetStateTable())
				return
			}
			if consumedCount.Load() != int64(seeds) {
				t.Errorf("Expected %d seeds to be consumed, got %d", seeds, consumedCount.Load())
				return
			}
			Stop()
			log.Stop()
			t.Errorf("Timeout waiting for reactor to finish processing")
			return
		default:
			if len(GetStateTable()) == 0 {
				cancel()
				wg.Wait()
				if consumedCount.Load() != int64(seeds) {
					t.Errorf("Expected %d seeds to be consumed, got %d", seeds, consumedCount.Load())
					return
				}
				Stop()
				log.Stop()
				return
			}
		}
	}
}
