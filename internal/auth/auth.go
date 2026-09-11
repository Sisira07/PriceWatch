// Package auth provides password hashing (bcrypt) and JWT issuance/verification.
// It has no database dependency, so it can be fully unit tested in isolation.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters")
	ErrEmailRequired      = errors.New("email is required")
	ErrMismatchedPassword = errors.New("invalid email or password")
)

const minPasswordLength = 8

// Claims is the JWT payload used across the API.
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// TokenIssuer issues and verifies JWTs signed with a shared HMAC secret.
type TokenIssuer struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenIssuer(secret string, ttl time.Duration) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret), ttl: ttl}
}

// HashPassword validates and bcrypt-hashes a plaintext password.
func HashPassword(password string) (string, error) {
	if len(password) < minPasswordLength {
		return "", ErrPasswordTooShort
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword compares a plaintext password against a bcrypt hash.
func CheckPassword(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrMismatchedPassword
	}
	return nil
}

// ValidateEmail applies a minimal sanity check; deeper validation (MX lookup
// etc.) is deliberately out of scope for this project.
func ValidateEmail(email string) error {
	if email == "" {
		return ErrEmailRequired
	}
	at := -1
	for i, c := range email {
		if c == '@' {
			at = i
			break
		}
	}
	if at <= 0 || at == len(email)-1 {
		return fmt.Errorf("%q is not a valid email", email)
	}
	return nil
}

// IssueToken creates a signed JWT for the given user.
func (t *TokenIssuer) IssueToken(userID, email string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(t.ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(t.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// VerifyToken parses and validates a JWT, returning its claims.
func (t *TokenIssuer) VerifyToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(tok *jwt.Token) (interface{}, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return t.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}