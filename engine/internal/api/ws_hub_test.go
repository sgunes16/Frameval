package api

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestHub_BroadcastNeverBlocksWhenChannelFull(t *testing.T) {
	// Deliberately do NOT call hub.Run — the broadcast channel never
	// drains, so it fills after 256 sends. The 5,000-message producer
	// must still return; non-blocking Broadcast is the invariant under
	// test. Without it, this goroutine would deadlock on send #257.
	hub := NewHub()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 5_000; i++ {
			hub.Broadcast("test.event", map[string]int{"i": i})
		}
		close(done)
	}()

	select {
	case <-done:
		// success — producer finished, broadcast did not block on the
		// saturated channel.
	case <-time.After(2 * time.Second):
		t.Fatal("Broadcast appears to be blocking on a full channel")
	}

	// 5,000 sends into a 256-slot channel means ~4,744 should have been
	// dropped; allow some slack for the (in practice none, since Run is
	// not started) channel drain rate.
	dropped := hub.DroppedBroadcasts()
	if dropped < 4_000 {
		t.Errorf("expected ≥ 4,000 dropped broadcasts when channel saturated, got %d", dropped)
	}
}

func TestHub_BroadcastReachesRegisteredClient(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := make(chan []byte, 4)
	hub.register <- client

	// Tiny payload, single broadcast — must arrive at the client.
	hub.Broadcast("hello", map[string]string{"k": "v"})

	select {
	case msg := <-client:
		if len(msg) == 0 {
			t.Error("client received an empty payload")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("client did not receive the broadcast")
	}
}

func TestHub_SlowClientDoesNotBlockOtherClients(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	slow := make(chan []byte, 1) // capacity 1 — saturates immediately
	fast := make(chan []byte, 64)
	// register is unbuffered: each send blocks until Run's select arm
	// receives it, so by the time the second send returns both clients
	// are guaranteed to be in the hub's client map.
	hub.register <- slow
	hub.register <- fast

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			hub.Broadcast("x", i)
		}
	}()
	wg.Wait()

	// fast must have received SOME messages; if the slow client
	// dragged everyone down, fast would be empty or near-empty.
	received := len(fast)
	if received == 0 {
		t.Error("fast client received nothing — slow client appears to be blocking the hub")
	}
}
