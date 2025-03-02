package tokens

import "time"

// Config holds the configuration for token generation and validation
type Config struct {
	// AccessTokenSecret is the secret key for signing access tokens
	AccessTokenSecret string
	// RefreshTokenSecret is the secret key for signing refresh tokens
	RefreshTokenSecret string
	// AccessTokenDuration is the validity duration for access tokens
	AccessTokenDuration time.Duration
	// RefreshTokenDuration is the validity duration for refresh tokens
	RefreshTokenDuration time.Duration
}

// DefaultConfig returns a Config with reasonable default values
func DefaultConfig() Config {
	return Config{
		AccessTokenDuration:  15 * time.Minute,   // 15 minutes
		RefreshTokenDuration: 7 * 24 * time.Hour, // 7 days
	}
}

// WithAccessTokenSecret sets the access token secret
func (c Config) WithAccessTokenSecret(secret string) Config {
	c.AccessTokenSecret = secret
	return c
}

// WithRefreshTokenSecret sets the refresh token secret
func (c Config) WithRefreshTokenSecret(secret string) Config {
	c.RefreshTokenSecret = secret
	return c
}

// WithAccessTokenDuration sets the access token duration
func (c Config) WithAccessTokenDuration(duration time.Duration) Config {
	c.AccessTokenDuration = duration
	return c
}

// WithRefreshTokenDuration sets the refresh token duration
func (c Config) WithRefreshTokenDuration(duration time.Duration) Config {
	c.RefreshTokenDuration = duration
	return c
}
