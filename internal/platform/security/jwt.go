// Package security contains password and JWT helpers used by the application.
package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTManager signs and validates JWT access tokens.
type JWTManager struct {
	secret   []byte
	issuer   string
	audience string
	now      func() time.Time
}

type accessClaims struct {
	jwt.RegisteredClaims
}

// NewJWTManager creates a token manager with the required claims.
func NewJWTManager(secret, issuer, audience string) *JWTManager {
	return &JWTManager{secret: []byte(secret), issuer: issuer, audience: audience, now: time.Now}
}

// Issue creates a signed JWT for a user.
func (m *JWTManager) Issue(subject string, ttl time.Duration) (string, error) {
	if subject == "" {
		return "", errors.New("subject is required")
	}
	if ttl <= 0 {
		return "", errors.New("ttl must be positive")
	}
	var tokenID [16]byte
	if _, err := rand.Read(tokenID[:]); err != nil {
		return "", fmt.Errorf("generate token ID: %w", err)
	}
	now := m.now().UTC()
	claims := accessClaims{RegisteredClaims: jwt.RegisteredClaims{
		Issuer:    m.issuer,
		Audience:  jwt.ClaimStrings{m.audience},
		Subject:   subject,
		ID:        hex.EncodeToString(tokenID[:]),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// Validate checks a JWT signature and required claims.
func (m *JWTManager) Validate(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &accessClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(m.issuer), jwt.WithAudience(m.audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil {
		return "", err
	}
	claims, ok := token.Claims.(*accessClaims)
	if !ok || !token.Valid {
		return "", errors.New("invalid token")
	}
	if claims.Subject == "" || claims.ID == "" || claims.IssuedAt == nil || claims.ExpiresAt == nil {
		return "", errors.New("missing subject claim")
	}
	return claims.Subject, nil
}
