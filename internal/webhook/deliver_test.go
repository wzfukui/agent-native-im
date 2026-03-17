package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wzfukui/agent-native-im/internal/model"
)

// testDelays returns minimal delays suitable for tests.
func testDelays() []time.Duration {
	return []time.Duration{0, 1 * time.Millisecond, 1 * time.Millisecond}
}

// newTestDeliverer creates a Deliverer with fast retries and no store (not needed for unit tests).
func newTestDeliverer() *Deliverer {
	return &Deliverer{
		client:      &http.Client{Timeout: 5 * time.Second},
		RetryDelays: testDelays(),
	}
}

func testMessage() *model.Message {
	return &model.Message{
		ID:             1,
		ConversationID: 10,
		SenderID:       100,
		Layers:         model.MessageLayers{Summary: "hello"},
	}
}

func TestDeliverSuccess(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := newTestDeliverer()
	msg := testMessage()
	wh := &model.Webhook{
		ID:       1,
		EntityID: 42,
		URL:      srv.URL,
		Secret:   "test-secret",
		Events:   []string{"message.new"},
	}

	ctx := context.Background()
	d.deliverToWebhook(ctx, wh, msg)

	if got := calls.Load(); got != 1 {
		t.Errorf("expected 1 call, got %d", got)
	}
}

func TestDeliverRetryOnFailure(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := newTestDeliverer()
	msg := testMessage()
	wh := &model.Webhook{
		ID:       1,
		EntityID: 42,
		URL:      srv.URL,
		Secret:   "test-secret",
		Events:   []string{"message.new"},
	}

	ctx := context.Background()
	d.deliverToWebhook(ctx, wh, msg)

	if got := calls.Load(); got != 3 {
		t.Errorf("expected 3 calls (2 failures + 1 success), got %d", got)
	}
}

func TestDeliverMaxRetriesExhausted(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := newTestDeliverer()
	msg := testMessage()
	wh := &model.Webhook{
		ID:       1,
		EntityID: 42,
		URL:      srv.URL,
		Secret:   "test-secret",
		Events:   []string{"message.new"},
	}

	ctx := context.Background()
	d.deliverToWebhook(ctx, wh, msg)

	if got := calls.Load(); got != 3 {
		t.Errorf("expected 3 calls (all failed), got %d", got)
	}
}

func TestDeliverHMACSignature(t *testing.T) {
	msg := testMessage()
	secret := "my-webhook-secret"

	body, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}

	// Compute expected signature.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	var receivedSig string
	var receivedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Signature")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := newTestDeliverer()
	wh := &model.Webhook{
		ID:       1,
		EntityID: 42,
		URL:      srv.URL,
		Secret:   secret,
		Events:   []string{"message.new"},
	}

	ctx := context.Background()
	d.deliverToWebhook(ctx, wh, msg)

	if receivedSig != expectedSig {
		t.Errorf("signature mismatch\n  got:  %s\n  want: %s", receivedSig, expectedSig)
	}

	// Verify the body matches what we signed.
	if string(receivedBody) != string(body) {
		t.Errorf("body mismatch\n  got:  %s\n  want: %s", receivedBody, body)
	}
}

func TestDeliverTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		// Delay longer than the client timeout (200ms > 100ms).
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := &Deliverer{
		client:      &http.Client{Timeout: 100 * time.Millisecond},
		RetryDelays: []time.Duration{0, 1 * time.Millisecond, 1 * time.Millisecond},
	}

	msg := testMessage()
	wh := &model.Webhook{
		ID:       1,
		EntityID: 42,
		URL:      srv.URL,
		Secret:   "test-secret",
		Events:   []string{"message.new"},
	}

	ctx := context.Background()
	d.deliverToWebhook(ctx, wh, msg)

	// All 3 attempts should have been made (and all timed out).
	if got := calls.Load(); got != 3 {
		t.Errorf("expected 3 timeout attempts, got %d", got)
	}
}

func TestSignFunction(t *testing.T) {
	body := []byte(`{"test":"data"}`)
	secret := "secret123"

	got := sign(body, secret)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))

	if got != want {
		t.Errorf("sign() = %s, want %s", got, want)
	}
}
