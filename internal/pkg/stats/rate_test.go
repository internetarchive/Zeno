package stats

import (
	"testing"
	"time"
)

func TestRate_Start(t *testing.T) {
	rate := &rate{}

	// Increment the rate counter
	rate.incr(5)

	// Wait for more than a second to allow the ticker to update the rate
	time.Sleep(1100 * time.Millisecond)

	// Check if the rate per second is correctly updated
	gotRate := rate.get()
	if gotRate != 5 {
		t.Errorf("expected rate per second to be 5, got %d", gotRate)
	}

	// Increment the rate counter again
	rate.incr(10)

	// Wait for more than a second to allow the ticker to update the rate
	time.Sleep(1100 * time.Millisecond)

	// Check if the rate per second is correctly updated
	gotRate = rate.get()
	if gotRate != 10 {
		t.Errorf("expected rate per second to be 10, got %d", gotRate)
	}

	// Increment the rate counter multiple times and check the rate over several seconds
	for i := 0; i < 5; i++ {
		rate.incr(2)
		time.Sleep(1100 * time.Millisecond)
		expectedRate := uint64(2)
		gotRate = rate.get()
		if gotRate != expectedRate {
			t.Errorf("expected rate per second to be %d, got %d", expectedRate, gotRate)
		}
	}
}

func TestRate_Incr(t *testing.T) {
	rate := &rate{}

	// Increment the rate counter
	rate.incr(3)

	// Check if the count is correctly incremented
	if rate.count.Load() != 3 {
		t.Errorf("expected count to be 3, got %d", rate.count.Load())
	}

	// Increment the rate counter again
	rate.incr(2)

	// Check if the count is correctly incremented
	if rate.count.Load() != 5 {
		t.Errorf("expected count to be 5, got %d", rate.count.Load())
	}
}

func TestRate_Get(t *testing.T) {
	rate := &rate{}

	// Increment the rate counter
	rate.incr(7)

	// Wait for more than a second to allow the ticker to update the rate
	time.Sleep(1100 * time.Millisecond)

	// Check if the rate per second is correctly updated
	if rate.get() != 7 {
		t.Errorf("expected rate per second to be 7, got %d", rate.get())
	}
}

func TestRate_GetTotal(t *testing.T) {
	rate := &rate{}

	// Fuzz the rate counter
	rate.incr(7)
	time.Sleep(1 * time.Second)
	rate.get()
	rate.incr(3)
	time.Sleep(1 * time.Second)
	rate.get()
	rate.incr(0)

	// Check if the total count is correctly retrieved
	if rate.getTotal() != 10 {
		t.Errorf("expected total count to be 10, got %d", rate.getTotal())
	}
}
