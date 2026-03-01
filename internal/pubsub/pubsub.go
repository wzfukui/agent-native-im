package pubsub

import "context"

// Event is a message published on a PubSub channel.
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// PubSub abstracts publish/subscribe for real-time message delivery.
// Day-1: in-memory implementation. Future: Redis, NATS, etc.
type PubSub interface {
	// Publish sends an event to all subscribers of the channel.
	Publish(ctx context.Context, channel string, event Event) error

	// Subscribe returns a channel that receives events and a cancel function.
	Subscribe(ctx context.Context, channel string) (events <-chan Event, cancel func(), err error)
}
