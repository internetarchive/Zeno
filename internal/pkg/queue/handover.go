package queue

import (
	"sync/atomic"
)

type HandoverChannel struct {
	ch    chan *handoverEncodedItem
	count *atomic.Uint64
	open  *atomic.Bool
}

type handoverEncodedItem struct {
	bytes []byte
	item  *Item
}

func NewHandoverChannel() *HandoverChannel {
	handover := &HandoverChannel{
		ch:    make(chan *handoverEncodedItem),
		count: new(atomic.Uint64),
		open:  new(atomic.Bool),
	}
	handover.open.Store(false)
	close(handover.ch)
	return handover
}

func (h *HandoverChannel) TryOpen(size int) bool {
	if h.open.Load() {
		return false
	}
	h.ch = make(chan *handoverEncodedItem, size)
	h.open.Store(true)
	return true
}

func (h *HandoverChannel) TryClose() bool {
	if !h.open.Load() {
		return false
	}
	close(h.ch)
	h.open.Store(false)
	return true
}

func (h *HandoverChannel) TryPut(item *handoverEncodedItem) bool {
	select {
	case h.ch <- item:
		return true
	default:
		return false
	}
}

func (h *HandoverChannel) TryGet() (*handoverEncodedItem, bool) {
	select {
	case item := <-h.ch:
		h.count.Add(1)
		return item, true
	default:
		return nil, false
	}
}

func (h *HandoverChannel) Drain()
