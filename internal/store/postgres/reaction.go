package postgres

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/wzfukui/agent-native-im/internal/model"
)

func (s *PGStore) AddReaction(ctx context.Context, r *model.Reaction) error {
	_, err := s.DB.NewInsert().Model(r).
		On("CONFLICT (message_id, entity_id, emoji) DO NOTHING").
		Exec(ctx)
	return err
}

func (s *PGStore) RemoveReaction(ctx context.Context, messageID, entityID int64, emoji string) error {
	_, err := s.DB.NewDelete().Model((*model.Reaction)(nil)).
		Where("message_id = ?", messageID).
		Where("entity_id = ?", entityID).
		Where("emoji = ?", emoji).
		Exec(ctx)
	return err
}

func (s *PGStore) GetReactionsByMessages(ctx context.Context, messageIDs []int64) (map[int64][]model.ReactionSummary, error) {
	if len(messageIDs) == 0 {
		return nil, nil
	}

	var rows []struct {
		MessageID int64  `bun:"message_id"`
		Emoji     string `bun:"emoji"`
		EntityID  int64  `bun:"entity_id"`
	}

	err := s.DB.NewSelect().
		TableExpr("reactions").
		Column("message_id", "emoji", "entity_id").
		Where("message_id IN (?)", bun.In(messageIDs)).
		OrderExpr("message_id, emoji, entity_id").
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}

	// Aggregate into summaries
	result := make(map[int64][]model.ReactionSummary)
	type key struct {
		msgID int64
		emoji string
	}
	idx := make(map[key]int) // key → index in result[msgID]

	for _, r := range rows {
		k := key{r.MessageID, r.Emoji}
		if i, ok := idx[k]; ok {
			result[r.MessageID][i].Count++
			result[r.MessageID][i].EntityIDs = append(result[r.MessageID][i].EntityIDs, r.EntityID)
		} else {
			idx[k] = len(result[r.MessageID])
			result[r.MessageID] = append(result[r.MessageID], model.ReactionSummary{
				Emoji:     r.Emoji,
				Count:     1,
				EntityIDs: []int64{r.EntityID},
			})
		}
	}

	return result, nil
}
