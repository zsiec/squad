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

	for i := 0; i < subBuffer+8; i++ {
		bus.Publish(Event{Kind: "saturate"})
	}
	if got := len(slow); got != subBuffer {
		t.Fatalf("slow sub len=%d want %d after saturation", got, subBuffer)
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
		case e := <-fast:
			if e.Kind == "after-saturation" {
				received++
			}
		case <-deadline:
			t.Fatalf("fast sub only got %d/%d after slow saturation", received, n)
		}
	}
}

// Reproducer: publisher bursts past the buffer, then stops.
// The publish-side lag injection only fires on the NEXT publish — if no
// more come, drops stay invisible to the subscriber. PullDropped lets a
// consumer (the SSE ping tick) surface the count without depending on
// future publishes.
func TestBus_PullDropped_VisibleWithoutFurtherPublish(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	sub := bus.Subscribe()
	defer bus.Unsubscribe(sub)

	const overflow = 200
	for i := 0; i < subBuffer+overflow; i++ {
		bus.Publish(Event{Kind: "msg"})
	}
	// Publisher quiesces. Consumer drains nothing yet.
	got := bus.PullDropped(sub)
	if got < overflow {
		t.Fatalf("PullDropped=%d want >=%d (drops should be visible without further publishes)", got, overflow)
	}
	// Counter must be reset by the swap.
	if again := bus.PullDropped(sub); again != 0 {
		t.Fatalf("PullDropped not reset: %d", again)
	}
}

func TestBus_PullDropped_UnknownChannelReturnsZero(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	stranger := make(chan Event)
	if got := bus.PullDropped(stranger); got != 0 {
		t.Fatalf("PullDropped on unknown channel = %d, want 0", got)
	}
}

func TestBus_LagSentinel_AfterDrops(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	sub := bus.Subscribe()
	defer bus.Unsubscribe(sub)

	const overflow = 50
	for i := 0; i < subBuffer+overflow; i++ {
		bus.Publish(Event{Kind: "msg"})
	}
	for len(sub) > 0 {
		<-sub
	}

	bus.Publish(Event{Kind: "msg"})

	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case e := <-sub:
			if e.Kind == "lag" {
				if dropped, ok := e.Payload["dropped"].(int64); !ok || dropped < overflow {
					t.Fatalf("lag payload=%v want dropped>=%d", e.Payload, overflow)
				}
				return
			}
		case <-deadline:
			t.Fatal("never received lag sentinel")
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
