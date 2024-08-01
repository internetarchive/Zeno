package queue

import (
	"sync/atomic"
	"time"
)

type handoverChannel struct {
	ch                  chan *handoverEncodedItem
	open                *atomic.Bool
	ready               *atomic.Bool
	drained             *atomic.Bool
	signalConsumerDone  chan bool
	activityTracker     *activityTracker
	activityTrackerSize int
	monitorInterval     *atomic.Value // stores time.Duration
}

type handoverEncodedItem struct {
	bytes []byte
	item  *Item
}

type activityTracker struct {
	buffer []*int64
	size   int
	head   atomic.Uint64
}

func newActivityTracker(size int) *activityTracker {
	buffer := make([]*int64, size)
	for i := range buffer {
		buffer[i] = new(int64)
	}
	return &activityTracker{
		buffer: buffer,
		size:   size,
	}
}

func (at *activityTracker) record(value int64) {
	head := at.head.Add(1) % uint64(at.size)
	atomic.StoreInt64(at.buffer[head], value)
}

func (at *activityTracker) sum() int64 {
	var sum int64
	for _, v := range at.buffer {
		sum += atomic.LoadInt64(v)
	}
	return sum
}

func newHandoverChannel() *handoverChannel {
	handover := &handoverChannel{
		ch:                  make(chan *handoverEncodedItem),
		open:                new(atomic.Bool),
		ready:               new(atomic.Bool),
		drained:             new(atomic.Bool),
		signalConsumerDone:  make(chan bool, 1),
		monitorInterval:     new(atomic.Value),
		activityTrackerSize: 5, // 5 intervals
	}
	handover.monitorInterval.Store(10 * time.Millisecond)
	handover.open.Store(false)
	close(handover.ch)
	return handover
}

func (h *handoverChannel) tryOpen(size int) bool {
	if !h.open.CompareAndSwap(false, true) {
		return false
	}
	h.ch = make(chan *handoverEncodedItem, size)
	h.open.Store(true)
	h.ready.Store(true)
	h.drained.Store(false)
	h.activityTracker = newActivityTracker(h.activityTrackerSize)
	go h.monitorActivity()
	return true
}

func (h *handoverChannel) tryClose() bool {
	if !h.open.CompareAndSwap(true, false) || h.ready.Load() || h.drained.Load() {
		return false
	}
	select {
	case item := <-h.ch:
		if item != nil {
			panic("handover channel not drained") // This should never happen
		}
	default:
		break
	}
	close(h.ch)
	h.activityTracker = nil
	h.monitorInterval.Store(10 * time.Millisecond)
	return true
}

func (h *handoverChannel) tryPut(item *handoverEncodedItem) bool {
	if !h.ready.Load() || !h.open.Load() {
		return false
	}
	select {
	case h.ch <- item:
		return true
	default:
		return false
	}
}

func (h *handoverChannel) tryGet() (*handoverEncodedItem, bool) {
	if !h.ready.Load() || !h.open.Load() {
		return nil, false
	}
	select {
	case item := <-h.ch:
		if item == nil {
			return nil, true
		}
		h.activityTracker.record(1)
		return item, true
	default:
		return nil, false
	}
}

func (h *handoverChannel) tryDrain() ([]*handoverEncodedItem, bool) {
	if !h.open.Load() {
		return nil, false
	}

	items := []*handoverEncodedItem{}
	ok := false
	for {
		select {
		case item := <-h.ch:
			if item == nil {
				continue
			}
			items = append(items, item)
			ok = true
		default:
			h.drained.Store(true)
			return items, ok
		}
	}
}

func (h *handoverChannel) monitorActivity() {
	for h.open.Load() {
		interval := h.monitorInterval.Load().(time.Duration)
		time.Sleep(interval)

		activity := h.activityTracker.sum()
		h.activityTracker.record(0)

		if activity == 0 {
			h.ready.Store(false)
			// Grace period
			time.Sleep(20 * time.Millisecond)
			if h.activityTracker.sum() == 0 {
				h.signalConsumerDone <- true
				return
			}
			h.ready.Store(true)
		}

		// Adjust monitoring interval based on activity
		if activity > 10 {
			h.monitorInterval.Store(20 * time.Millisecond)
		} else if activity < 2 {
			h.monitorInterval.Store(5 * time.Millisecond)
		}
	}
}
