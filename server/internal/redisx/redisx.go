package redisx

import "github.com/redis/go-redis/v9"

func New(url string) (*redis.Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	opt.PoolSize = 20
	return redis.NewClient(opt), nil
}
