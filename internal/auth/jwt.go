package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/wzfukui/agent-native-im/internal/model"
)

type Claims struct {
	EntityID   int64            `json:"entity_id"`
	EntityType model.EntityType `json:"entity_type"`
	jwt.RegisteredClaims
}

func GenerateToken(secret string, entityID int64, entityType model.EntityType) (string, error) {
	claims := Claims{
		EntityID:   entityID,
		EntityType: entityType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseToken(secret, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}
