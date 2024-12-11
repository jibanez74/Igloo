package main

import (
	"errors"
	"igloo/cmd/helpers"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v4"
)

func (app *config) isAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Authorization")

		cookie, err := r.Cookie("token")
		if err != nil {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				helpers.ErrorJSON(w, errors.New("unauthorized"), http.StatusUnauthorized)
				return
			}

			headerParts := strings.Split(authHeader, " ")
			if len(headerParts) != 2 || headerParts[0] != "Bearer" {
				helpers.ErrorJSON(w, errors.New("unauthorized"), http.StatusUnauthorized)
				return
			}

			cookie = &http.Cookie{Value: headerParts[1]}
		}

		token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}

			return []byte(app.cookieSecret), nil
		})

		if err != nil {
			helpers.ErrorJSON(w, errors.New("unauthorized"), http.StatusUnauthorized)
			return
		}

		if !token.Valid {
			helpers.ErrorJSON(w, errors.New("invalid token"), http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			helpers.ErrorJSON(w, errors.New("invalid token claims"), http.StatusUnauthorized)
			return
		}

		if claims["aud"] != helpers.Aud || claims["iss"] != helpers.Iss {
			helpers.ErrorJSON(w, errors.New("invalid token claims"), http.StatusUnauthorized)
			return
		}

		userID, ok := claims["sub"].(string)
		if ok {
			ctx := helpers.ContextWithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		helpers.ErrorJSON(w, errors.New("invalid user ID in token"), http.StatusUnauthorized)
	})
}
