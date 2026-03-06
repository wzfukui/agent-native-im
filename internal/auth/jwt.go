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
	return GenerateTokenWithTTL(secret, entityID, entityType, 24*time.Hour)
}

func GenerateTokenWithTTL(secret string, entityID int64, entityType model.EntityType, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	claims := Claims{
		EntityID:   entityID,
		EntityType: entityType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
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

// ParseTokenAllowExpired validates token signature but skips exp/nbf/iat claim checks.
// Use only for refresh flows where recently expired JWTs may be accepted.
func ParseTokenAllowExpired(secret, tokenStr string) (*Claims, error) {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	token, err := parser.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
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
