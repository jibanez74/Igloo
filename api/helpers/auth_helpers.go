// description: helper functions to work with authentication
package helpers

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenPairs struct {
	Token        string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func createToken(id uint, secretKey, issuer, audience string, expirationTime int64) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["sub"] = fmt.Sprint(id)
	claims["aud"] = audience
	claims["iss"] = issuer
	claims["iat"] = time.Now().UTC().Unix()
	claims["exp"] = time.Now().UTC().Add(time.Second * time.Duration(expirationTime)).Unix()

	signedToken, err := token.SignedString([]byte(secretKey))

	if err != nil {
		return "", err
	}

	return signedToken, nil
}

func GenerateTokenPair(id uint) (TokenPairs, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	jwtIssuer := os.Getenv("JWT_ISSUER")
	jwtAudience := os.Getenv("JWT_AUDIENCE")
	jwtExpiration, _ := strconv.ParseInt(os.Getenv("JWT_EXPIRATION"), 10, 64)
	jwtRefreshExpiration, _ := strconv.ParseInt(os.Getenv("JWT_REFRESH_EXPIRATION"), 10, 64)

	signedAccessToken, err := createToken(id, jwtSecret, jwtIssuer, jwtAudience, jwtExpiration)
	if err != nil {
		return TokenPairs{}, err
	}

	signedRefreshToken, err := createToken(id, jwtSecret, jwtIssuer, jwtAudience, jwtRefreshExpiration)
	if err != nil {
		return TokenPairs{}, err
	}

	return TokenPairs{
		Token:        signedAccessToken,
		RefreshToken: signedRefreshToken,
	}, nil
}

func GetRefreshCookie(refreshToken string) *http.Cookie {
	cookieName := os.Getenv("COOKIE_NAME")
	cookiePath := os.Getenv("COOKIE_PATH")
	refreshExpiryStr := os.Getenv("JWT_REFRESH_EXPIRATION")
	cookieDomain := os.Getenv("COOKIE_DOMAIN")

	refreshExpiry, _ := strconv.Atoi(refreshExpiryStr)

	return &http.Cookie{
		Name:     cookieName,
		Path:     cookiePath,
		Value:    refreshToken,
		Expires:  time.Now().Add(time.Duration(refreshExpiry) * time.Second),
		MaxAge:   refreshExpiry,
		SameSite: http.SameSiteStrictMode,
		Domain:   cookieDomain,
		HttpOnly: true,
		Secure:   true,
	}
}
