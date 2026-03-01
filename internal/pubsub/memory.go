package pubsub

import (
	"context"
	"sync"
)

type subscriber struct {
	ch     chan Event
	cancel func()
}

// MemoryPubSub is an in-process PubSub using Go channels.
type MemoryPubSub struct {
	mu   sync.RWMutex
	subs map[string][]*subscriber
}

func NewMemory() *MemoryPubSub {
	return &MemoryPubSub{
		subs: make(map[string][]*subscriber),
	}
}

func (m *MemoryPubSub) Publish(_ context.Context, channel string, event Event) error {
	m.mu.RLock()
	subs := m.subs[channel]
	m.mu.RUnlock()

	for _, s := range subs {
		select {
		case s.ch <- event:
		default:
			// subscriber buffer full, skip
		}
	}
	return nil
}

func (m *MemoryPubSub) Subscribe(_ context.Context, channel string) (<-chan Event, func(), error) {
	ch := make(chan Event, 64)

	sub := &subscriber{ch: ch}
	sub.cancel = func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		subs := m.subs[channel]
		for i, s := range subs {
			if s == sub {
				m.subs[channel] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		close(ch)
	}

	m.mu.Lock()
	m.subs[channel] = append(m.subs[channel], sub)
	m.mu.Unlock()

	return ch, sub.cancel, nil
}
