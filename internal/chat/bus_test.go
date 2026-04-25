package chat

import (
	"sync"
	"testing"
	"time"
)

func TestBus_PublishToSubscriber(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	bus.Publish(Event{Kind: "message", Payload: map[string]any{"thread": "global"}})

	select {
	case e := <-ch:
		if e.Kind != "message" {
			t.Fatalf("kind=%q", e.Kind)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("no event received")
	}
}

func TestBus_FanOutToMultiple(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	subs := []chan Event{bus.Subscribe(), bus.Subscribe(), bus.Subscribe()}
	defer func() {
		for _, s := range subs {
			bus.Unsubscribe(s)
		}
	}()

	bus.Publish(Event{Kind: "claim"})

	var wg sync.WaitGroup
	for i, s := range subs {
		wg.Add(1)
		go func(i int, ch chan Event) {
			defer wg.Done()
			select {
			case e := <-ch:
				if e.Kind != "claim" {
					t.Errorf("sub %d got %q", i, e.Kind)
				}
			case <-time.After(100 * time.Millisecond):
				t.Errorf("sub %d: no event", i)
			}
		}(i, s)
	}
	wg.Wait()
}

func TestBus_DoesNotBlockOnSlowSubscriber(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	slow := bus.Subscribe()
	fast := bus.Subscribe()
	defer bus.Unsubscribe(slow)
	defer bus.Unsubscribe(fast)

	for i := 0; i < 40; i++ {
		bus.Publish(Event{Kind: "saturate"})
	}
	if got := len(slow); got != 32 {
		t.Fatalf("slow sub len=%d want 32 after saturation", got)
	}

	for len(fast) > 0 {
		<-fast
	}
	const n = 20
	for i := 0; i < n; i++ {
		bus.Publish(Event{Kind: "after-saturation"})
	}

	deadline := time.After(500 * time.Millisecond)
	received := 0
	for received < n {
		select {
		case <-fast:
			received++
		case <-deadline:
			t.Fatalf("fast sub only got %d/%d after slow saturation", received, n)
		}
	}
}

func TestBus_SubscribeAfterClose_ReturnsClosedChannel(t *testing.T) {
	bus := NewBus()
	bus.Close()

	ch := bus.Subscribe()

	_, ok := <-ch
	if ok {
		t.Fatal("expected closed channel after Subscribe post-Close")
	}
}
