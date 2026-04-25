package chat

import "sync"

type Event struct {
	Kind    string         `json:"kind"`
	Payload map[string]any `json:"payload"`
}

type Bus struct {
	mu     sync.RWMutex
	subs   map[chan Event]struct{}
	closed bool
}

func NewBus() *Bus {
	return &Bus{subs: map[chan Event]struct{}{}}
}

func (b *Bus) Subscribe() chan Event {
	ch := make(chan Event, 32)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		close(ch)
		return ch
	}
	b.subs[ch] = struct{}{}
	return ch
}

func (b *Bus) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	if _, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(ch)
	}
	b.mu.Unlock()
}

func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return
	}
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for ch := range b.subs {
		close(ch)
	}
	b.subs = nil
}
