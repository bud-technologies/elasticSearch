package elastic

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"github.com/patrickmn/go-cache"
	"time"
)

type Cache interface {
	Get(md5Id string) (any, CacheError)
	Put(md5Id string, info any, Expiration time.Duration) error
}

type CacheError interface {
	Error() string
	IsTimeOutErr() bool
	IsNotFoundErr() bool
}

var (
	esCache = cache.New(15*time.Second, 30*time.Second)
)

func GetDefaultCache() Cache {
	return defaultCache{cache: esCache}
}

var _ Cache = (*defaultCache)(nil)

type defaultCache struct {
	cache *cache.Cache
}

func (d defaultCache) Get(md5Id string) (any, CacheError) {
	result, ok := d.cache.Get(md5Id)
	if !ok {
		return result, defaultCacheErr{errData: "cache not found"}
	} else {
		return result, nil
	}
}

func (d defaultCache) Put(md5Id string, info any, Expiration time.Duration) error {
	if Expiration < 0 {
		return errors.New("can't set 0 expiration")
	}
	d.cache.Set(md5Id, info, Expiration)
	return nil
}

var _ CacheError = (*defaultCacheErr)(nil)

type defaultCacheErr struct {
	errData string
}

func (d defaultCacheErr) Error() string {
	return d.errData
}

func (d defaultCacheErr) IsTimeOutErr() bool {
	return d.errData == "time out"
}

func (d defaultCacheErr) IsNotFoundErr() bool {
	return d.errData == "cache not found"

}

func str2md5(str []byte) string {
	h := md5.New()
	h.Write(str)
	return hex.EncodeToString(h.Sum(nil))
}
