package redisconn

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisURL(t *testing.T) {
	for _, tt := range []struct {
		in      *url.URL
		want    *RedisConnParams
		wantErr assert.ErrorAssertionFunc
	}{{
		in: mustParseURL("redis://localhost:6379"),
		want: &RedisConnParams{
			Scheme: "redis",
			Host:   "localhost",
			Port:   6379,
		},
	}, {
		in: mustParseURL("redis://localhost?cluster=true&dial_timeout=15m"),
		want: &RedisConnParams{
			Scheme:      "redis",
			Host:        "localhost",
			Port:        6379,
			Cluster:     true,
			DialTimeout: 15 * time.Minute,
		},
	}} {
		tt.wantErr = noErrAsDefault(tt.wantErr)

		t.Run("", func(t *testing.T) {
			got, err := parseRedisURL(tt.in)
			if !tt.wantErr(t, err) || err != nil {
				return
			}

			require.Equal(t, tt.want, got)
		})
	}
}

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}

	return u
}

func noErrAsDefault(t assert.ErrorAssertionFunc) assert.ErrorAssertionFunc {
	if t == nil {
		return assert.NoError
	}

	return t
}
