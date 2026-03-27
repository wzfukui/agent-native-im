package handler

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/wzfukui/agent-native-im/internal/model"
)

const entityPublicIDKey = "public_id"

func ensureEntityIdentity(entity *model.Entity) (changed bool, err error) {
	meta := map[string]any{}
	if len(entity.Metadata) > 0 {
		if err := json.Unmarshal(entity.Metadata, &meta); err != nil {
			return false, err
		}
	}

	storedMetaPublicID, _ := meta[entityPublicIDKey].(string)
	switch {
	case entity.PublicID != "":
		if storedMetaPublicID != entity.PublicID {
			meta[entityPublicIDKey] = entity.PublicID
			changed = true
		}
	case storedMetaPublicID != "":
		entity.PublicID = storedMetaPublicID
		changed = true
	default:
		entity.PublicID = uuid.NewString()
		meta[entityPublicIDKey] = entity.PublicID
		changed = true
	}

	if changed {
		encoded, err := json.Marshal(meta)
		if err != nil {
			return false, err
		}
		entity.Metadata = encoded
	}

	return changed, nil
}

func hydrateEntityIdentity(entity *model.Entity) {
	if entity == nil {
		return
	}
	if _, err := ensureEntityIdentity(entity); err != nil {
		slog.Warn("failed to hydrate entity identity", "entity_id", entity.ID, "error", err)
	}
}

func (s *Server) attachEntityIdentity(ctx context.Context, entity *model.Entity) {
	if entity == nil {
		return
	}
	changed, err := ensureEntityIdentity(entity)
	if err != nil {
		slog.Warn("failed to parse entity identity metadata", "entity_id", entity.ID, "error", err)
		return
	}
	if !changed {
		return
	}
	if err := s.Store.UpdateEntity(ctx, entity); err != nil {
		slog.Warn("failed to persist entity identity", "entity_id", entity.ID, "error", err)
	}
}

func (s *Server) attachEntitiesIdentity(ctx context.Context, entities []*model.Entity) {
	for _, entity := range entities {
		s.attachEntityIdentity(ctx, entity)
	}
}
