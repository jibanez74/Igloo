package tokens

import "errors"

var (
	// ErrInvalidToken indicates that the token is not valid
	ErrInvalidToken = errors.New("invalid token")

	// ErrExpiredToken indicates that the token has expired
	ErrExpiredToken = errors.New("token has expired")

	// ErrInvalidTokenType indicates that the token type is not valid
	ErrInvalidTokenType = errors.New("invalid token type")

	// ErrMissingSecret indicates that the secret key is missing
	ErrMissingSecret = errors.New("missing secret key")

	// ErrTokenRevoked indicates that the token has been revoked
	ErrTokenRevoked = errors.New("token has been revoked")

	// ErrInvalidClaims indicates that the token claims are invalid
	ErrInvalidClaims = errors.New("invalid token claims")
)
