package lock_test

import (
	"context"
	"errors"
	"testing"
	"time"

	businesslock "hestia/server/internal/common/lock"

	"github.com/redis/go-redis/v9"
)

func TestRedisLockerRunsFunctionAndReleasesOwnedLock(t *testing.T) {
	client := newFakeRedisLockClient()
	locker := businesslock.NewRedisLocker(client)
	called := false

	err := locker.WithLock(context.Background(), "hestia:lock:test", time.Minute, func(context.Context) error {
		called = true
		return nil
	})

	if err != nil {
		t.Fatalf("with lock: %v", err)
	}
	if !called {
		t.Fatal("expected locked function to run")
	}
	if _, ok := client.values["hestia:lock:test"]; ok {
		t.Fatalf("expected lock key to be released, got %#v", client.values)
	}
}

func TestRedisLockerReturnsBusyWhenLockAlreadyHeld(t *testing.T) {
	client := newFakeRedisLockClient()
	client.values["hestia:lock:test"] = "existing-token"
	locker := businesslock.NewRedisLocker(client)
	called := false

	err := locker.WithLock(context.Background(), "hestia:lock:test", time.Minute, func(context.Context) error {
		called = true
		return nil
	})

	if !errors.Is(err, businesslock.ErrBusy) {
		t.Fatalf("expected ErrBusy, got %v", err)
	}
	if called {
		t.Fatal("expected locked function not to run when lock is busy")
	}
	if client.values["hestia:lock:test"] != "existing-token" {
		t.Fatalf("expected existing token to remain, got %#v", client.values)
	}
}

func TestRedisLockerDoesNotReleaseLockOwnedByAnotherToken(t *testing.T) {
	client := newFakeRedisLockClient()
	locker := businesslock.NewRedisLocker(client)

	err := locker.WithLock(context.Background(), "hestia:lock:test", time.Minute, func(context.Context) error {
		client.values["hestia:lock:test"] = "other-token"
		return errors.New("business failed")
	})

	if err == nil {
		t.Fatal("expected business error")
	}
	if client.values["hestia:lock:test"] != "other-token" {
		t.Fatalf("expected other token to remain, got %#v", client.values)
	}
}

func TestRedisLockerIgnoresReleaseErrorAfterBusinessSuccess(t *testing.T) {
	client := newFakeRedisLockClient()
	client.releaseErr = errors.New("redis release unavailable")
	locker := businesslock.NewRedisLocker(client)
	called := false

	err := locker.WithLock(context.Background(), "hestia:lock:test", time.Minute, func(context.Context) error {
		called = true
		return nil
	})

	if err != nil {
		t.Fatalf("expected business success to win over release error, got %v", err)
	}
	if !called {
		t.Fatal("expected locked function to run")
	}
}

func TestRedisLockerReturnsNoopWhenClientIsNil(t *testing.T) {
	locker := businesslock.NewRedisLocker(nil)
	called := false

	err := locker.WithLock(context.Background(), "hestia:lock:test", time.Minute, func(context.Context) error {
		called = true
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil redis client to use noop locker, got %v", err)
	}
	if !called {
		t.Fatal("expected noop lock to run function")
	}
}

type fakeRedisLockClient struct {
	values     map[string]string
	releaseErr error
}

func newFakeRedisLockClient() *fakeRedisLockClient {
	return &fakeRedisLockClient{values: map[string]string{}}
}

func (c *fakeRedisLockClient) SetNX(_ context.Context, key string, value any, _ time.Duration) *redis.BoolCmd {
	if _, exists := c.values[key]; exists {
		return redis.NewBoolResult(false, nil)
	}
	c.values[key] = value.(string)
	return redis.NewBoolResult(true, nil)
}

func (c *fakeRedisLockClient) Eval(_ context.Context, _ string, keys []string, args ...any) *redis.Cmd {
	if c.releaseErr != nil {
		return redis.NewCmdResult(nil, c.releaseErr)
	}
	key := keys[0]
	expected := args[0].(string)
	if c.values[key] == expected {
		delete(c.values, key)
		return redis.NewCmdResult(int64(1), nil)
	}
	return redis.NewCmdResult(int64(0), nil)
}
