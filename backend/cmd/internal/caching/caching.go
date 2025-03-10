package caching

import (
	"os"
	"strconv"

	"github.com/gofiber/storage/redis/v3"
)

func New() *redis.Storage {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}

	redisPort, err := strconv.Atoi(os.Getenv("REDIS_PORT"))
	if err != nil {
		redisPort = 6379 // Default Redis port
	}

	store := redis.New(redis.Config{
		Host:     host,
		Port:     redisPort,
		Username: os.Getenv("REDIS_USERNAME"),
		Password: os.Getenv("REDIS_PASSWORD"),
		Database: 0,
		Reset:    false,
	})

	return store
}
