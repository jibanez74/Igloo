package tokens

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type jwtManager struct {
	config Config
}

// customClaims extends jwt.RegisteredClaims with our custom Claims
type customClaims struct {
	Claims
	jwt.RegisteredClaims
}

// New creates a new TokenManager implementation
func New(config Config) (TokenManager, error) {
	if config.AccessTokenSecret == "" {
		return nil, fmt.Errorf("access token: %w", ErrMissingSecret)
	}

	if config.RefreshTokenSecret == "" {
		return nil, fmt.Errorf("refresh token: %w", ErrMissingSecret)
	}

	return &jwtManager{config: config}, nil
}

func (m *jwtManager) GenerateTokenPair(claims Claims) (*TokenPair, error) {
	accessToken, err := m.generateToken(claims, AccessToken, m.config.AccessTokenSecret, m.config.AccessTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}

	refreshToken, err := m.generateToken(claims, RefreshToken, m.config.RefreshTokenSecret, m.config.RefreshTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("generating refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(m.config.AccessTokenDuration),
	}, nil
}

func (m *jwtManager) ValidateToken(tokenString string, tokenType TokenType) (*Claims, error) {
	var secret string

	switch tokenType {
	case AccessToken:
		secret = m.config.AccessTokenSecret
	case RefreshToken:
		secret = m.config.RefreshTokenSecret
	default:
		return nil, ErrInvalidTokenType
	}

	token, err := jwt.ParseWithClaims(tokenString, &customClaims{}, func(token *jwt.Token) (interface{}, error) {
		_, ok := token.Method.(*jwt.SigningMethodHMAC)
		if !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secret), nil
	})

	if err != nil {
		if err == jwt.ErrTokenExpired {
			return nil, ErrExpiredToken
		}

		return nil, fmt.Errorf("parsing token: %w", ErrInvalidToken)
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*customClaims)
	if !ok {
		return nil, ErrInvalidClaims
	}

	if claims.TokenType != tokenType {
		return nil, ErrInvalidTokenType
	}

	return &claims.Claims, nil
}

func (m *jwtManager) RefreshTokens(refreshToken string) (*TokenPair, error) {
	claims, err := m.ValidateToken(refreshToken, RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("validating refresh token: %w", err)
	}

	return m.GenerateTokenPair(*claims)
}

// RevokeToken implements TokenManager.RevokeToken
// Note: This is a basic implementation. In a production environment,
// you might want to maintain a blacklist of revoked tokens in a database or Redis
func (m *jwtManager) RevokeToken(token string) error {
	// TODO: Implement token revocation logic
	// This could involve adding the token to a blacklist in a database or Redis
	return nil
}

// generateToken creates a new JWT token with the given claims and configuration
func (m *jwtManager) generateToken(claims Claims, tokenType TokenType, secret string, duration time.Duration) (string, error) {
	now := time.Now()
	claims.TokenType = tokenType

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, customClaims{
		Claims: claims,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	})

	return token.SignedString([]byte(secret))
}
