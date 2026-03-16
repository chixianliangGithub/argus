package eventbus

import (
	"context"
	"errors"
	"sync"
)

// Event represents a generic event.
type Event struct {
	Topic string      `json:"topic"`
	Data  interface{} `json:"data"`
}

// EventHandler is a function that handles events.
type EventHandler func(ctx context.Context, event Event) error

// EventBus defines the interface for an event bus.
type EventBus interface {
	Publish(ctx context.Context, topic string, data interface{}) error
	Subscribe(topic string, handler EventHandler) error
	Close()
}

// MemoryEventBus is a simple in-memory event bus implementation using Go channels.
type MemoryEventBus struct {
	handlers map[string][]EventHandler
	mu       sync.RWMutex
	queue    chan Event
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewMemoryEventBus() *MemoryEventBus {
	ctx, cancel := context.WithCancel(context.Background())
	bus := &MemoryEventBus{
		handlers: make(map[string][]EventHandler),
		queue:    make(chan Event, 100), // Buffered channel
		ctx:      ctx,
		cancel:   cancel,
	}
	go bus.processEvents()
	return bus
}

func (b *MemoryEventBus) Publish(ctx context.Context, topic string, data interface{}) error {
	select {
	case b.queue <- Event{Topic: topic, Data: data}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-b.ctx.Done():
		return errors.New("event bus closed")
	}
}

func (b *MemoryEventBus) Subscribe(topic string, handler EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], handler)
	return nil
}

func (b *MemoryEventBus) processEvents() {
	for {
		select {
		case <-b.ctx.Done():
			return
		case event, ok := <-b.queue:
			if !ok {
				return
			}
			b.mu.RLock()
			handlers, ok := b.handlers[event.Topic]
			b.mu.RUnlock()
			if ok {
				for _, handler := range handlers {
					go func(h EventHandler, e Event) {
						_ = h(context.Background(), e)
					}(handler, event)
				}
			}
		}
	}
}

func (b *MemoryEventBus) Close() {
	b.cancel()
}
