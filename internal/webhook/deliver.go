package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/wzfukui/agent-native-im/internal/model"
	"github.com/wzfukui/agent-native-im/internal/store"
)

// Deliverer handles async webhook delivery with retry.
type Deliverer struct {
	store  store.Store
	client *http.Client
}

// NewDeliverer creates a webhook deliverer.
func NewDeliverer(st store.Store) *Deliverer {
	return &Deliverer{
		store: st,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// DeliverAsync sends webhooks for a message to all relevant subscribers.
func (d *Deliverer) DeliverAsync(msg *model.Message) {
	go func() {
		ctx := context.Background()

		webhooks, err := d.store.GetWebhooksForConversation(ctx, msg.ConversationID, "message.new")
		if err != nil {
			log.Printf("webhook: failed to get webhooks for conversation %d: %v", msg.ConversationID, err)
			return
		}

		for _, wh := range webhooks {
			// Don't deliver to the sender's own webhook
			if wh.EntityID == msg.SenderID {
				continue
			}
			d.deliverToWebhook(wh, msg)
		}
	}()
}

func (d *Deliverer) deliverToWebhook(wh *model.Webhook, msg *model.Message) {
	body, err := json.Marshal(msg)
	if err != nil {
		log.Printf("webhook: failed to marshal message: %v", err)
		return
	}

	signature := sign(body, wh.Secret)

	// Retry schedule: 0s, 5s, 25s
	delays := []time.Duration{0, 5 * time.Second, 25 * time.Second}
	for attempt, delay := range delays {
		if delay > 0 {
			time.Sleep(delay)
		}

		err = d.deliver(wh.URL, body, signature, wh.EntityID)
		if err == nil {
			log.Printf("webhook: delivered to entity %d (attempt %d)", wh.EntityID, attempt+1)
			return
		}
		log.Printf("webhook: attempt %d failed for entity %d: %v", attempt+1, wh.EntityID, err)
	}

	log.Printf("webhook: all retries exhausted for entity %d", wh.EntityID)
}

func (d *Deliverer) deliver(url string, body []byte, signature string, entityID int64) error {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", "sha256="+signature)
	req.Header.Set("X-Entity-ID", fmt.Sprintf("%d", entityID))

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("http status %d", resp.StatusCode)
}

func sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
