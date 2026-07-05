package jwtutil

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type claims struct {
	UserID uint `json:"user_id"`
	jwt.RegisteredClaims
}

func GenerateToken(userID uint, secret string, expiryHours int) (string, error) {
	c := claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiryHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(secret))
}

// ParseToken returns the token's user ID and IssuedAt time — the latter is
// needed by middleware.JWTAuth to support "log out of all devices" (see
// model.User.TokenValidAfter).
func ParseToken(tokenStr, secret string) (uint, time.Time, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return 0, time.Time{}, fmt.Errorf("invalid token")
	}

	c, ok := token.Claims.(*claims)
	if !ok {
		return 0, time.Time{}, fmt.Errorf("invalid claims")
	}

	var issuedAt time.Time
	if c.IssuedAt != nil {
		issuedAt = c.IssuedAt.Time
	}
	return c.UserID, issuedAt, nil
}
