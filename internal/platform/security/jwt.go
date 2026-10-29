// Package security contains password and JWT helpers used by the application.
package security

import (
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
}

// NewJWTManager creates a token manager with the required claims.
func NewJWTManager(secret, issuer, audience string) *JWTManager {
	return &JWTManager{secret: []byte(secret), issuer: issuer, audience: audience}
}

// Issue creates a signed JWT for a user.
func (m *JWTManager) Issue(subject string, ttl time.Duration) (string, error) {
	if subject == "" {
		return "", errors.New("subject is required")
	}
	if ttl <= 0 {
		return "", errors.New("ttl must be positive")
	}
	claims := jwt.MapClaims{
		"iss": m.issuer,
		"aud": m.audience,
		"sub": subject,
		"iat": time.Now().UTC().Unix(),
		"exp": time.Now().Add(ttl).UTC().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// Validate checks a JWT signature and required claims.
func (m *JWTManager) Validate(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return "", err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", errors.New("invalid token")
	}
	if claims["iss"] != m.issuer || claims["aud"] != m.audience {
		return "", errors.New("token claims invalid")
	}
	if claims["sub"] == nil {
		return "", errors.New("missing subject claim")
	}
	return fmt.Sprint(claims["sub"]), nil
}
