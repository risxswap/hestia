package lock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrBusy = errors.New("business lock is busy")

type Locker interface {
	WithLock(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) error) error
}

type NoopLocker struct{}

func (NoopLocker) WithLock(ctx context.Context, _ string, _ time.Duration, fn func(context.Context) error) error {
	return fn(ctx)
}

type RedisLocker struct {
	client redisLockClient
}

type redisLockClient interface {
	SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd
	Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd
}

func NewRedisLocker(client redisLockClient) *RedisLocker {
	return &RedisLocker{client: client}
}

func (l *RedisLocker) WithLock(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) error) error {
	if l == nil || l.client == nil {
		return fn(ctx)
	}
	if ttl <= 0 {
		return fmt.Errorf("business lock ttl must be positive")
	}
	token, err := randomToken()
	if err != nil {
		return err
	}
	acquired, err := l.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return err
	}
	if !acquired {
		return ErrBusy
	}
	fnErr := fn(ctx)
	releaseErr := l.client.Eval(ctx, releaseScript, []string{key}, token).Err()
	if fnErr != nil {
		return fnErr
	}
	_ = releaseErr
	return nil
}

func randomToken() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

const releaseScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`
