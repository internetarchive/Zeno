package queue

import "sync/atomic"

type HandoverChannel struct {
	ch    chan *Item
	count *atomic.Uint64
}

func NewHandoverChannel() *HandoverChannel {
	return &HandoverChannel{
		ch:    make(chan *Item, 1), // Buffer of 1 for non-blocking operations
		count: new(atomic.Uint64),
	}
}

func (h *HandoverChannel) TryPut(item *Item) bool {
	select {
	case h.ch <- item:
		return true
	default:
		return false
	}
}

func (h *HandoverChannel) TryGet() (*Item, bool) {
	select {
	case item := <-h.ch:
		h.count.Add(1)
		return item, true
	default:
		return nil, false
	}
}
