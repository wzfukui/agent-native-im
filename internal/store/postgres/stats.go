package postgres

import (
	"context"
)

func (s *PGStore) GetSystemStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count entities by type
	var userCount int
	userCount, err := s.DB.NewSelect().TableExpr("entities").
		Where("entity_type = ?", "user").Where("status = ?", "active").Count(ctx)
	if err != nil {
		return nil, err
	}
	stats["user_count"] = userCount

	var botCount int
	botCount, err = s.DB.NewSelect().TableExpr("entities").
		Where("entity_type = ?", "bot").Where("status = ?", "active").Count(ctx)
	if err != nil {
		return nil, err
	}
	stats["bot_count"] = botCount

	var convCount int
	convCount, err = s.DB.NewSelect().TableExpr("conversations").Count(ctx)
	if err != nil {
		return nil, err
	}
	stats["conversation_count"] = convCount

	var msgCount int
	msgCount, err = s.DB.NewSelect().TableExpr("messages").Count(ctx)
	if err != nil {
		return nil, err
	}
	stats["message_count"] = msgCount

	return stats, nil
}
