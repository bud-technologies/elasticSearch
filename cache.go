package elastic

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"time"
)

type Cache interface {
	Get(ctx context.Context, esType string) (string, CacheError)
	Put(ctx context.Context, esType string, info string, Expiration time.Duration) error
}

type CacheError interface {
	Error() string
	IsTimeOutErr() bool
	IsNotFoundErr() bool
}

func str2md5(str []byte) string {
	h := md5.New()
	h.Write(str)
	return hex.EncodeToString(h.Sum(nil))
}

// getMd5Id
func getMd5Id(server any) string {
	return str2md5([]byte(fmt.Sprintln(server)))
}
