package chat

import (
	"sync"
	"sync/atomic"
)

type Event struct {
	Kind    string         `json:"kind"`
	Payload map[string]any `json:"payload"`
}

// subBuffer sized for realistic burst load. Earlier 32-element buffer caused
// silent SSE event loss when a single pump tick batched more rows than that
// or when an HTTP flush trailed the publish rate; QA round 5 D-1 measured
// ~22% loss on a 500-msg burst, ~82% loss on a 1000-msg/5-sub fan-out.
const subBuffer = 1024

type subscriber struct {
	ch      chan Event
	dropped atomic.Int64
}

type Bus struct {
	mu     sync.RWMutex
	subs   map[chan Event]*subscriber
	closed bool
}

func NewBus() *Bus {
	return &Bus{subs: map[chan Event]*subscriber{}}
}

func (b *Bus) Subscribe() chan Event {
	s := &subscriber{ch: make(chan Event, subBuffer)}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		close(s.ch)
		return s.ch
	}
	b.subs[s.ch] = s
	return s.ch
}

func (b *Bus) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if s, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(s.ch)
	}
}

func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return
	}
	for _, s := range b.subs {
		// If this subscriber has dropped events, try to inject a lag sentinel
		// before resuming normal flow. dropped.Swap(0) atomically claims the
		// count so concurrent publishers don't double-count or wipe each
		// other's drops; if our send fails we put the count back. The
		// `dropped` payload is best-effort — clients must treat `lag` as a
		// "refetch from MAX(id)" trigger, not an exact loss count.
		if n := s.dropped.Swap(0); n > 0 {
			select {
			case s.ch <- Event{Kind: "lag", Payload: map[string]any{"dropped": n}}:
				// success; count consumed by this sentinel
			default:
				s.dropped.Add(n + 1) // restore plus this publish (also dropped)
				continue
			}
		}
		select {
		case s.ch <- e:
		default:
			s.dropped.Add(1)
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
	for _, s := range b.subs {
		close(s.ch)
	}
	b.subs = nil
}
