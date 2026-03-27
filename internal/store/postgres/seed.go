package postgres

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/wzfukui/agent-native-im/internal/model"
)

// SeedAdmin creates the admin user entity with a password credential.
// Idempotent — skips if the entity already exists.
func (s *PGStore) SeedAdmin(ctx context.Context, username, password string) error {
	// Check if already exists
	_, err := s.GetEntityByName(ctx, username, model.EntityUser)
	if err == nil {
		return nil // already exists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	entity := &model.Entity{
		PublicID:    uuid.NewString(),
		EntityType:  model.EntityUser,
		Name:        username,
		DisplayName: username,
		Status:      "active",
	}
	if err := s.CreateEntity(ctx, entity); err != nil {
		return fmt.Errorf("create entity: %w", err)
	}

	cred := &model.Credential{
		EntityID:   entity.ID,
		CredType:   model.CredPassword,
		SecretHash: string(hash),
		RawPrefix:  fmt.Sprintf("%x", sha256.Sum256([]byte(password)))[:8],
	}
	if err := s.CreateCredential(ctx, cred); err != nil {
		return fmt.Errorf("create credential: %w", err)
	}

	slog.Info("seed: admin user created", "username", username, "id", entity.ID)
	return nil
}
