package helpers

import (
	"errors"
	"fmt"
	"igloo/cmd/database/models"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const tokenExp = time.Minute * 15
const refreshExp = time.Hour * 24 * 7
const aud = "igloo"
const iss = "igloo"
const cookieName = "igloo"

func GenerateToken(user *models.User, secret string) (TokenPairs, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["sub"] = fmt.Sprint(user.ID)
	claims["aud"] = aud
	claims["iss"] = iss
	claims["iat"] = time.Now().UTC().Unix()
	claims["typ"] = "JWT"
	claims["exp"] = time.Now().UTC().Add(tokenExp).Unix()

	signedAccessToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return TokenPairs{}, err
	}

	refreshToken := jwt.New(jwt.SigningMethodHS256)
	refreshTokenClaims := refreshToken.Claims.(jwt.MapClaims)
	refreshTokenClaims["sub"] = fmt.Sprint(user.ID)
	refreshTokenClaims["iat"] = time.Now().UTC().Unix()
	refreshTokenClaims["exp"] = time.Now().UTC().Add(refreshExp).Unix()

	signedRefreshToken, err := refreshToken.SignedString([]byte(secret))
	if err != nil {
		return TokenPairs{}, err
	}

	tokens := TokenPairs{
		Token:        signedAccessToken,
		RefreshToken: signedRefreshToken,
	}

	return tokens, nil
}

func SetRefreshCookie(token, cookieDomain string) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Path:     "/",
		Value:    token,
		Expires:  time.Now().Add(refreshExp),
		MaxAge:   int(refreshExp.Seconds()),
		SameSite: http.SameSiteStrictMode,
		Domain:   cookieDomain,
		HttpOnly: true,
		Secure:   true,
	}
}

func SetExpiredCookie(cookieDomain string) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Path:     "/",
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		SameSite: http.SameSiteStrictMode,
		Domain:   cookieDomain,
		HttpOnly: true,
		Secure:   true,
	}
}

func GetTokenFromHeaderAndVerify(w http.ResponseWriter, r *http.Request, secret string) (string, *Claims, error) {
	w.Header().Add("Vary", "Authorization")

	authHeader := r.Header.Get("Authorization")

	if authHeader == "" {
		return "", nil, errors.New("no auth header")
	}

	headerParts := strings.Split(authHeader, " ")
	if len(headerParts) != 2 {
		return "", nil, errors.New("invalid auth header")
	}

	if headerParts[0] != "Bearer" {
		return "", nil, errors.New("invalid auth header")
	}

	token := headerParts[1]

	claims := &Claims{}

	_, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		_, ok := token.Method.(*jwt.SigningMethodHMAC)
		if !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secret), nil
	})

	if err != nil {
		if strings.HasPrefix(err.Error(), "token is expired by") {
			return "", nil, errors.New("expired token")
		}

		return "", nil, err
	}

	if claims.Issuer != iss {
		return "", nil, errors.New("invalid issuer")
	}

	return token, claims, nil
}
