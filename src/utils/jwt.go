package utils

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"hng-s1/src/data"

	"github.com/golang-jwt/jwt/v4"
)

var (
	AccessExpiry  = 15 * time.Minute
	RefreshExpiry = 7 * 24 * time.Hour
)

type Claims struct {
	UserID string    `json:"uid"`
	Role   data.Role `json:"role"`
	jwt.RegisteredClaims
}

type ClaimsKey struct{}

func ClaimsFromCtx(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(ClaimsKey{}).(*Claims)
	return c, ok
}

func NewAccessToken(userID string, role data.Role, secret []byte) (string, error) {
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

func ParseAccessToken(tokenStr string, secret []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func NewRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
