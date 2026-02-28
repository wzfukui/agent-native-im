package store

import (
	"context"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	user := new(model.User)
	err := s.DB.NewSelect().Model(user).Where("username = ?", username).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	user := new(model.User)
	err := s.DB.NewSelect().Model(user).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return user, nil
}
