package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hestia/server/internal/common/auth"

	"github.com/gin-gonic/gin"
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
