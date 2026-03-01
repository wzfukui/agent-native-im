package postgres

import (
	"context"

	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) CreateCredential(ctx context.Context, cred *model.Credential) error {
	_, err := s.DB.NewInsert().Model(cred).Exec(ctx)
	return err
}

func (s *PGStore) GetCredentialsByEntity(ctx context.Context, entityID int64, credType model.CredType) ([]*model.Credential, error) {
	var creds []*model.Credential
	err := s.DB.NewSelect().Model(&creds).
		Where("entity_id = ?", entityID).
		Where("cred_type = ?", credType).
		Scan(ctx)
	return creds, err
}

func (s *PGStore) GetCredentialByPrefix(ctx context.Context, credType model.CredType, prefix string) ([]*model.Credential, error) {
	var creds []*model.Credential
	err := s.DB.NewSelect().Model(&creds).
		Where("cred_type = ?", credType).
		Where("raw_prefix = ?", prefix).
		Scan(ctx)
	return creds, err
}

func (s *PGStore) DeleteCredentialsByEntity(ctx context.Context, entityID int64) error {
	_, err := s.DB.NewDelete().Model((*model.Credential)(nil)).
		Where("entity_id = ?", entityID).
		Exec(ctx)
	return err
}
