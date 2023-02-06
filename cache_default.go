package elastic

import (
	"context"
	"errors"
	"github.com/go-redis/redis/v8"
	"time"
)

func GetDefaultCache(redisClient *redis.ClusterClient, key string) Cache {
	return defaultCache{client: redisClient, cacheKey: key}
}

var _ Cache = (*defaultCache)(nil)

type defaultCache struct {
	client   *redis.ClusterClient
	cacheKey string
}

func (d defaultCache) Get(ctx context.Context, esType string) (string, CacheError) {
	cacheStr, err := d.client.Get(ctx, getKeyIdKey(d.cacheKey, esType)).Result()
	if err != nil {
		return cacheStr, defaultCacheErr{errData: err}
	}
	return cacheStr, nil
}

func (d defaultCache) Put(ctx context.Context, esType string, info string, Expiration time.Duration) error {
	if Expiration < 0 {
		return errors.New("can't set 0 expiration")
	}

	return d.client.Set(ctx, getKeyIdKey(d.cacheKey, esType), info, Expiration).Err()
}

var _ CacheError = (*defaultCacheErr)(nil)

type defaultCacheErr struct {
	errData error
}

func (d defaultCacheErr) Error() string {
	if d.errData == nil {
		return ""
	}
	return d.errData.Error()
}

func (d defaultCacheErr) IsTimeOutErr() bool {
	if d.errData == nil {
		return false
	}
	return d.errData == redis.Nil
}

func (d defaultCacheErr) IsNotFoundErr() bool {
	if d.errData == nil {
		return false
	}
	return d.errData == redis.Nil
}

func getKeyIdKey(cacheKey string, esType string) string {
	return "esGet:" + esType + ":" + cacheKey
}
