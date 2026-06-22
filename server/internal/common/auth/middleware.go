package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"hestia/server/internal/common/response"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const sessionKeyPrefix = "hestia:user-session:"

var ErrSessionNotFound = errors.New("session not found")

type Session struct {
	UserID       int64     `json:"user_id"`
	UserPublicID string    `json:"user_public_id"`
	Surface      string    `json:"surface"`
	CreatedAt    time.Time `json:"created_at"`
}

type SessionStore interface {
	Get(ctx context.Context, token string) (Session, error)
}

type SessionWriter interface {
	Set(ctx context.Context, token string, session Session, ttl time.Duration) error
}

type redisSessionClient interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
}

type RedisSessionStore struct {
	client redisSessionClient
}

func NewRedisSessionStore(client redisSessionClient) *RedisSessionStore {
	return &RedisSessionStore{client: client}
}

func (s *RedisSessionStore) Get(ctx context.Context, token string) (Session, error) {
	if s == nil || s.client == nil {
		return Session{}, ErrSessionNotFound
	}
	raw, err := s.client.Get(ctx, sessionKey(token)).Bytes()
	if errors.Is(err, redis.Nil) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, err
	}
	var session Session
	if err := json.Unmarshal(raw, &session); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *RedisSessionStore) Set(ctx context.Context, token string, session Session, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return ErrSessionNotFound
	}
	raw, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, sessionKey(token), raw, ttl).Err()
}

func RequireUserSession(store SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			unauthorized(c)
			return
		}
		session, err := store.Get(c.Request.Context(), token)
		if err != nil {
			unauthorized(c)
			return
		}
		if session.Surface != "user" {
			unauthorized(c)
			return
		}
		SetUserContext(c, User{
			UserID:       session.UserID,
			UserPublicID: session.UserPublicID,
			Surface:      session.Surface,
		})
		c.Next()
	}
}

func bearerToken(header string) string {
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(token)
}

func sessionKey(token string) string {
	return sessionKeyPrefix + token
}

func unauthorized(c *gin.Context) {
	response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
	c.Abort()
}
