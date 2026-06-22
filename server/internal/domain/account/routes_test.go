package account_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hestia/server/internal/common/auth"
	"hestia/server/internal/domain/account"

	"github.com/gin-gonic/gin"
)

type fakeUserRepo struct {
	user account.User
}

func (r *fakeUserRepo) UpsertDevUser(_ context.Context, input account.DevUserInput) (account.User, error) {
	r.user = account.User{
		ID:               12,
		PublicID:         input.PublicID,
		WechatOpenID:     input.WechatOpenID,
		Nickname:         input.Nickname,
		OnboardingStatus: "not_started",
		Status:           "active",
	}
	return r.user, nil
}

type fakeSessionWriter struct {
	token   string
	session auth.Session
	ttl     time.Duration
}

func (w *fakeSessionWriter) Set(_ context.Context, token string, session auth.Session, ttl time.Duration) error {
	w.token = token
	w.session = session
	w.ttl = ttl
	return nil
}

func TestDevLoginCreatesUserAndReturnsToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeUserRepo{}
	writer := &fakeSessionWriter{}
	service := account.NewService(repo, writer, time.Hour)
	router := gin.New()
	account.RegisterRoutes(router.Group("/api/user"), account.NewHandler(service))
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/user/dev-login",
		strings.NewReader(`{"nickname":"测试用户","dev_key":"ming-local"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	var body struct {
		Code string `json:"code"`
		Data struct {
			UserPublicID     string `json:"user_public_id"`
			Token            string `json:"token"`
			OnboardingStatus string `json:"onboarding_status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "ok" {
		t.Fatalf("expected code ok, got %q", body.Code)
	}
	if body.Data.UserPublicID == "" {
		t.Fatalf("expected user_public_id")
	}
	if !strings.HasPrefix(body.Data.Token, "dev_") {
		t.Fatalf("expected dev_ token, got %q", body.Data.Token)
	}
	if repo.user.WechatOpenID != "dev:ming-local" {
		t.Fatalf("expected dev openid, got %q", repo.user.WechatOpenID)
	}
	if writer.session.UserID != 12 || writer.session.UserPublicID != body.Data.UserPublicID {
		t.Fatalf("expected session user, got %#v", writer.session)
	}
	if writer.token != body.Data.Token {
		t.Fatalf("expected writer token %q, got %q", body.Data.Token, writer.token)
	}
}
