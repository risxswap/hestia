package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hestia/server/internal/common/auth"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type fakeSessionStore struct {
	session auth.Session
	err     error
}

func (s fakeSessionStore) Get(_ context.Context, _ string) (auth.Session, error) {
	if s.err != nil {
		return auth.Session{}, s.err
	}
	return s.session, nil
}

func TestRequireUserSessionRejectsMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/me", auth.RequireUserSession(fakeSessionStore{}), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

func TestRequireUserSessionStoresUserContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := fakeSessionStore{
		session: auth.Session{
			UserID:       12,
			UserPublicID: "usr_test",
			Surface:      "user",
			CreatedAt:    time.Now(),
		},
	}
	router.GET("/me", auth.RequireUserSession(store), func(c *gin.Context) {
		user, ok := auth.UserFromContext(c)
		if !ok || user.UserID != 12 || user.UserPublicID != "usr_test" {
			t.Fatalf("expected user context, got %#v", user)
		}
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	request.Header.Set("Authorization", "Bearer dev_token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
}

func TestRequireUserSessionRejectsUnknownToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/me", auth.RequireUserSession(fakeSessionStore{err: errors.New("missing")}), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	request.Header.Set("Authorization", "Bearer dev_token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

func TestRequireUserSessionRejectsNonUserSurface(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := fakeSessionStore{
		session: auth.Session{
			UserID:       12,
			UserPublicID: "usr_test",
			Surface:      "other",
			CreatedAt:    time.Now(),
		},
	}
	router.GET("/me", auth.RequireUserSession(store), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	request.Header.Set("Authorization", "Bearer other_token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

type fakeRedisClient struct {
	values   map[string][]byte
	setKey   string
	setValue any
	setTTL   time.Duration
}

func newFakeRedisClient() *fakeRedisClient {
	return &fakeRedisClient{values: map[string][]byte{}}
}

func (c *fakeRedisClient) Get(_ context.Context, key string) *redis.StringCmd {
	value, ok := c.values[key]
	if !ok {
		return redis.NewStringResult("", redis.Nil)
	}
	return redis.NewStringResult(string(value), nil)
}

func (c *fakeRedisClient) Set(_ context.Context, key string, value any, ttl time.Duration) *redis.StatusCmd {
	c.setKey = key
	c.setValue = value
	c.setTTL = ttl
	switch raw := value.(type) {
	case []byte:
		c.values[key] = raw
	case string:
		c.values[key] = []byte(raw)
	default:
		encoded, _ := json.Marshal(raw)
		c.values[key] = encoded
	}
	return redis.NewStatusResult("OK", nil)
}

func TestRedisSessionStoreSetWritesExpectedKeyAndJSON(t *testing.T) {
	client := newFakeRedisClient()
	store := auth.NewRedisSessionStore(client)
	createdAt := time.Date(2026, 6, 22, 10, 30, 0, 0, time.UTC)

	err := store.Set(context.Background(), "dev_token", auth.Session{
		UserID:       12,
		UserPublicID: "usr_test",
		Surface:      "user",
		CreatedAt:    createdAt,
	}, time.Hour)

	if err != nil {
		t.Fatalf("set session: %v", err)
	}
	if client.setKey != "hestia:user-session:dev_token" {
		t.Fatalf("expected session key, got %q", client.setKey)
	}
	if client.setTTL != time.Hour {
		t.Fatalf("expected ttl %s, got %s", time.Hour, client.setTTL)
	}
	var payload map[string]any
	if err := json.Unmarshal(client.values[client.setKey], &payload); err != nil {
		t.Fatalf("decode stored JSON: %v", err)
	}
	if payload["user_id"] != float64(12) {
		t.Fatalf("expected user_id 12, got %#v", payload["user_id"])
	}
	if payload["user_public_id"] != "usr_test" {
		t.Fatalf("expected user_public_id usr_test, got %#v", payload["user_public_id"])
	}
	if payload["surface"] != "user" {
		t.Fatalf("expected surface user, got %#v", payload["surface"])
	}
	if payload["created_at"] == "" {
		t.Fatalf("expected created_at in stored JSON")
	}
}

func TestRedisSessionStoreGetReadsJSONSession(t *testing.T) {
	client := newFakeRedisClient()
	store := auth.NewRedisSessionStore(client)
	createdAt := time.Date(2026, 6, 22, 10, 30, 0, 0, time.UTC)
	raw, err := json.Marshal(auth.Session{
		UserID:       12,
		UserPublicID: "usr_test",
		Surface:      "user",
		CreatedAt:    createdAt,
	})
	if err != nil {
		t.Fatalf("encode session: %v", err)
	}
	client.values["hestia:user-session:dev_token"] = raw

	session, err := store.Get(context.Background(), "dev_token")

	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.UserID != 12 {
		t.Fatalf("expected user_id 12, got %d", session.UserID)
	}
	if session.UserPublicID != "usr_test" {
		t.Fatalf("expected user_public_id usr_test, got %q", session.UserPublicID)
	}
	if session.Surface != "user" {
		t.Fatalf("expected surface user, got %q", session.Surface)
	}
	if !session.CreatedAt.Equal(createdAt) {
		t.Fatalf("expected created_at %s, got %s", createdAt, session.CreatedAt)
	}
}
