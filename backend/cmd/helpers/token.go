package helpers

import (
	"fmt"
	"igloo/cmd/database/models"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const LongLivedExp = time.Hour * 24 * 30 // 30 days
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
	claims["exp"] = time.Now().UTC().Add(LongLivedExp).Unix()
	claims["isAdmin"] = user.IsAdmin

	signedToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return signedToken, nil
}
