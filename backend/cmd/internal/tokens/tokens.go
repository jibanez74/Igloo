package tokens

import (
	"time"
)

// TokenType represents the type of token
type TokenType string

const (
	// AccessToken represents a short-lived token for API access
	AccessToken TokenType = "access"
	// RefreshToken represents a long-lived token for obtaining new access tokens
	RefreshToken TokenType = "refresh"
)

// Claims represents the custom claims in our JWT tokens
type Claims struct {
	UserID    int       `json:"user_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	IsAdmin   bool      `json:"is_admin"`
	TokenType TokenType `json:"token_type"`
}

// TokenPair represents a pair of access and refresh tokens
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// TokenManager interface defines the methods for token operations
type TokenManager interface {
	// GenerateTokenPair creates a new pair of access and refresh tokens
	GenerateTokenPair(claims Claims) (*TokenPair, error)

	// ValidateToken validates a token and returns its claims
	ValidateToken(token string, tokenType TokenType) (*Claims, error)

	// RefreshTokens generates new token pair using a valid refresh token
	RefreshTokens(refreshToken string) (*TokenPair, error)

	// RevokeToken invalidates a token before its expiration
	RevokeToken(token string) error
}
