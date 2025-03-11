package helpers

import (
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	jwt.RegisteredClaims
}

const expiresIn = 30 * time.Minute

func GenerateAccessToken(user *database.User, settings *database.GlobalSetting) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["sub"] = fmt.Sprint(user.ID)
	claims["aud"] = settings.Audience
	claims["iss"] = settings.Issuer
	claims["iat"] = time.Now().UTC().Unix()
	claims["typ"] = "JWT"

	claims["exp"] = time.Now().UTC().Add(expiresIn).Unix()

	signedAccessToken, err := token.SignedString([]byte(settings.Secret))
	if err != nil {
		return "", err
	}

	return signedAccessToken, nil
}

func VerifyAccessToken(token string, settings *database.GlobalSetting) error {
	claims := &Claims{}

	_, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		_, ok := token.Method.(*jwt.SigningMethodHMAC)
		if !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(settings.Secret), nil
	})

	if err != nil {
		return err
	}

	if claims.Issuer != settings.Issuer {
		return errors.New("invalid issuer")
	}

	return nil
}
