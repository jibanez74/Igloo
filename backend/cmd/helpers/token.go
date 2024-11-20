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

const longLivedExp = time.Hour * 24 * 30 // 30 days
const aud = "igloo"
const iss = "igloo"

func GenerateLongLivedToken(user *models.User, secret string) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["sub"] = fmt.Sprint(user.ID)
	claims["aud"] = aud
	claims["iss"] = iss
	claims["iat"] = time.Now().UTC().Unix()
	claims["typ"] = "JWT"
	claims["exp"] = time.Now().UTC().Add(longLivedExp).Unix()
	
	claims["name"] = user.Name
	claims["email"] = user.Email
	claims["username"] = user.Username
	claims["isAdmin"] = user.IsAdmin

	signedToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return signedToken, nil
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
