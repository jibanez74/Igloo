package caching

import "github.com/gomodule/redigo/redis"

func New(redisHost string) *redis.Pool {
	redisPool := &redis.Pool{
		MaxIdle: 10,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", redisHost)
		},
	}

	return redisPool
}
