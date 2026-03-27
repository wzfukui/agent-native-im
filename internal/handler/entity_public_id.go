package handler

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/wzfukui/agent-native-im/internal/model"
)

const entityPublicIDKey = "public_id"

func ensureMetadataPublicID(entity *model.Entity) (publicID string, changed bool, err error) {
	meta := map[string]any{}
	if len(entity.Metadata) > 0 {
		if err := json.Unmarshal(entity.Metadata, &meta); err != nil {
			return "", false, err
		}
	}

	if existing, ok := meta[entityPublicIDKey].(string); ok && existing != "" {
		entity.PublicID = existing
		return existing, false, nil
	}

	publicID = uuid.NewString()
	meta[entityPublicIDKey] = publicID
	encoded, err := json.Marshal(meta)
	if err != nil {
		return "", false, err
	}
	entity.Metadata = encoded
	entity.PublicID = publicID
	return publicID, true, nil
}

func hydrateEntityPublicID(entity *model.Entity) {
	if entity == nil {
		return
	}
	_, _, err := ensureMetadataPublicID(entity)
	if err != nil {
		slog.Warn("failed to hydrate entity public_id", "entity_id", entity.ID, "error", err)
	}
}

func (s *Server) attachEntityPublicID(ctx context.Context, entity *model.Entity) {
	if entity == nil {
		return
	}
	_, changed, err := ensureMetadataPublicID(entity)
	if err != nil {
		slog.Warn("failed to parse entity metadata for public_id", "entity_id", entity.ID, "error", err)
		return
	}
	if !changed {
		return
	}
	if err := s.Store.UpdateEntity(ctx, entity); err != nil {
		slog.Warn("failed to persist entity public_id", "entity_id", entity.ID, "error", err)
	}
}
