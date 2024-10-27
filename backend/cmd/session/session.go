package session

import (
	"os"
	"time"

	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/redis"
)

func New(port int) *session.Store {
	redisStore := redis.New(redis.Config{
		Host: os.Getenv("REDIS_HOST"),
		Port: port,
	})

	sessionStore := session.New(session.Config{
		Storage:        redisStore,
		CookieHTTPOnly: true, // More secure, prevents JavaScript access
		CookieSecure:   os.Getenv("MODE") == "prod",
		CookieSameSite: "Lax",
		KeyLookup:      "header:" + os.Getenv("SESSION_HEADER"),
		CookiePath:     os.Getenv("COOKIE_PATH"),
		CookieDomain:   os.Getenv("COOKIE_DOMAIN"),
		Expiration:     time.Hour * 24,
	})

	return sessionStore
}
