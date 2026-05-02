package watch

import (
	"sync"
	"testing"
	"time"
)

func TestHubFanOut(t *testing.T) {
	h := NewHub()

	a, unsubA := h.Subscribe()
	defer unsubA()
	b, unsubB := h.Subscribe()
	defer unsubB()

	want := Event{Cluster: "c", Kind: "Kustomization", Verb: "MODIFIED", Ns: "ns", Name: "n"}
	h.Publish(want)

	select {
	case got := <-a:
		if got != want {
			t.Errorf("a: got %+v want %+v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber a did not receive event")
	}
	select {
	case got := <-b:
		if got != want {
			t.Errorf("b: got %+v want %+v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber b did not receive event")
	}
}

func TestHubUnsubscribeStopsDelivery(t *testing.T) {
	h := NewHub()
	ch, unsub := h.Subscribe()
	unsub()

	// channel is closed; further publishes must not panic.
	h.Publish(Event{Kind: "X"})

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("read from closed channel did not return")
	}
}

func TestHubUnsubscribeIsIdempotent(t *testing.T) {
	h := NewHub()
	_, unsub := h.Subscribe()
	unsub()
	// Second call must not panic on the closed channel.
	unsub()
}

func TestHubDropsForSlowSubscriber(t *testing.T) {
	h := NewHub()
	ch, unsub := h.Subscribe()
	defer unsub()

	// The internal buffer is 64; publishing well past that without
	// reading must not block.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			h.Publish(Event{Kind: "X"})
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publishers blocked by slow subscriber")
	}

	// At least one event made it through before the buffer filled.
	select {
	case <-ch:
	default:
		t.Fatal("expected at least one event in subscriber buffer")
	}
}
