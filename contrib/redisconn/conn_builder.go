package redisconn

import (
	"net/url"

	"github.com/redis/go-redis/v9"
)

func setupClusterURL(u *url.URL) (*redis.ClusterOptions, error) {
	options, err := redis.ParseClusterURL(u.String())
	if err != nil {
		return nil, err
	}

	return options, nil
}
