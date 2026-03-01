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
	store  *store.Store
	client *http.Client
}

// NewDeliverer creates a webhook deliverer.
func NewDeliverer(s *store.Store) *Deliverer {
	return &Deliverer{
		store: s,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// DeliverAsync sends a webhook in a goroutine with retries.
// Only call this for user-sent messages (msg.SenderType == "user").
func (d *Deliverer) DeliverAsync(msg *model.Message) {
	go func() {
		ctx := context.Background()

		// Look up the bot for this conversation
		conv, err := d.store.GetConversation(ctx, msg.ConversationID)
		if err != nil {
			log.Printf("webhook: failed to get conversation %d: %v", msg.ConversationID, err)
			return
		}

		bot, err := d.store.GetBotByID(ctx, conv.BotID)
		if err != nil {
			log.Printf("webhook: failed to get bot %d: %v", conv.BotID, err)
			return
		}

		if bot.WebhookURL == "" {
			return // no webhook configured
		}

		body, err := json.Marshal(msg)
		if err != nil {
			log.Printf("webhook: failed to marshal message: %v", err)
			return
		}

		// Sign with HMAC-SHA256 using bot token as key
		signature := sign(body, bot.Token)

		// Retry schedule: 0s, 5s, 25s
		delays := []time.Duration{0, 5 * time.Second, 25 * time.Second}
		for attempt, delay := range delays {
			if delay > 0 {
				time.Sleep(delay)
			}

			err = d.deliver(bot.WebhookURL, body, signature, bot.ID)
			if err == nil {
				log.Printf("webhook: delivered to bot %d (attempt %d)", bot.ID, attempt+1)
				return
			}
			log.Printf("webhook: attempt %d failed for bot %d: %v", attempt+1, bot.ID, err)
		}

		log.Printf("webhook: all retries exhausted for bot %d", bot.ID)
	}()
}

func (d *Deliverer) deliver(url string, body []byte, signature string, botID int64) error {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", "sha256="+signature)
	req.Header.Set("X-Bot-ID", fmt.Sprintf("%d", botID))

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
