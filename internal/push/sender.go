package push

import (
	"context"
	"encoding/json"
	"log"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/wzfukui/agent-native-im/internal/config"
	"github.com/wzfukui/agent-native-im/internal/store"
)

// Sender handles sending Web Push notifications.
type Sender struct {
	store  store.Store
	config *config.Config
}

// NewSender creates a new push notification sender.
// Returns nil if VAPID keys are not configured.
func NewSender(st store.Store, cfg *config.Config) *Sender {
	if cfg.VAPIDPublicKey == "" || cfg.VAPIDPrivateKey == "" {
		return nil
	}
	return &Sender{store: st, config: cfg}
}

// Payload is the data sent in a push notification.
type Payload struct {
	Title          string `json:"title"`
	Body           string `json:"body"`
	ConversationID int64  `json:"conversation_id"`
	MessageID      int64  `json:"message_id"`
}

// SendToEntity sends push notifications to all subscribed devices of an entity.
func (s *Sender) SendToEntity(ctx context.Context, entityID int64, payload Payload) {
	subs, err := s.store.GetPushSubscriptionsByEntity(ctx, entityID)
	if err != nil || len(subs) == 0 {
		return
	}

	payloadBytes, _ := json.Marshal(payload)

	for _, sub := range subs {
		resp, err := webpush.SendNotification(payloadBytes, &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.KeyP256DH,
				Auth:   sub.KeyAuth,
			},
		}, &webpush.Options{
			VAPIDPublicKey:  s.config.VAPIDPublicKey,
			VAPIDPrivateKey: s.config.VAPIDPrivateKey,
			Subscriber:      s.config.VAPIDSubject,
		})
		if err != nil {
			log.Printf("push: failed to send to entity %d endpoint %s: %v", entityID, sub.Endpoint[:30], err)
			continue
		}
		resp.Body.Close()
		log.Printf("push: sent to entity %d, status=%d endpoint=%s", entityID, resp.StatusCode, sub.Endpoint[:40])

		// 410 Gone = subscription expired, remove it
		if resp.StatusCode == 410 {
			_ = s.store.DeletePushSubscription(ctx, entityID, sub.Endpoint)
			log.Printf("push: removed expired subscription for entity %d", entityID)
		}
	}
}
