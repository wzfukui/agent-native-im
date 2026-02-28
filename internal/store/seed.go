package store

import (
	"context"
	"log"

	"github.com/wzfukui/agent-native-im/internal/model"
	"golang.org/x/crypto/bcrypt"
)

func (s *Store) SeedAdmin(ctx context.Context, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := &model.User{
		Username:     username,
		PasswordHash: string(hash),
	}

	_, err = s.DB.NewInsert().
		Model(user).
		On("CONFLICT (username) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return err
	}

	log.Printf("Admin user '%s' ready", username)
	return nil
}
