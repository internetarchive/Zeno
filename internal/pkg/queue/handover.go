package queue

type HandoverChannel struct {
	ch chan *Item
}

func NewHandoverChannel() *HandoverChannel {
	return &HandoverChannel{
		ch: make(chan *Item, 1), // Buffer of 1 for non-blocking operations
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
		return item, true
	default:
		return nil, false
	}
}
