package eventbus

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestMemoryEventBus(t *testing.T) {
	bus := NewMemoryEventBus()
	defer bus.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	var receivedEvent Event
	handler := func(ctx context.Context, event Event) error {
		receivedEvent = event
		wg.Done()
		return nil
	}

	err := bus.Subscribe("test_topic", handler)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	err = bus.Publish(context.Background(), "test_topic", "test_data")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if receivedEvent.Topic != "test_topic" {
			t.Errorf("expected topic 'test_topic', got %q", receivedEvent.Topic)
		}
		if receivedEvent.Data != "test_data" {
			t.Errorf("expected data 'test_data', got %v", receivedEvent.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

func TestMemoryEventBus_MultipleSubscribers(t *testing.T) {
	bus := NewMemoryEventBus()
	defer bus.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	handler1 := func(ctx context.Context, event Event) error {
		wg.Done()
		return nil
	}

	handler2 := func(ctx context.Context, event Event) error {
		wg.Done()
		return nil
	}

	bus.Subscribe("multi_topic", handler1)
	bus.Subscribe("multi_topic", handler2)

	bus.Publish(context.Background(), "multi_topic", "data")

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for events")
	}
}
