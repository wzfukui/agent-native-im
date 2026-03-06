package auth

import (
	"testing"
	"time"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func TestGenerateTokenWithTTL(t *testing.T) {
	secret := "test-secret"
	start := time.Now()

	token, err := GenerateTokenWithTTL(secret, 123, model.EntityUser, 2*time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	claims, err := ParseToken(secret, token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}

	got := claims.ExpiresAt.Time.Sub(start)
	if got < 110*time.Minute || got > 130*time.Minute {
		t.Fatalf("unexpected ttl: %v", got)
	}
}
