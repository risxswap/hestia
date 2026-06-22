package account

import (
	"context"
	"errors"
	"strings"
	"time"

	"hestia/server/internal/common/auth"
	"hestia/server/internal/common/id"
)

const defaultSessionTTL = 30 * 24 * time.Hour

type Service struct {
	repo       UserRepository
	sessions   auth.SessionWriter
	sessionTTL time.Duration
}

func NewService(repo UserRepository, sessions auth.SessionWriter, sessionTTL time.Duration) *Service {
	if sessionTTL == 0 {
		sessionTTL = defaultSessionTTL
	}
	return &Service{repo: repo, sessions: sessions, sessionTTL: sessionTTL}
}

func (s *Service) DevLogin(ctx context.Context, input DevLoginInput) (DevLoginResult, error) {
	devKey := strings.TrimSpace(input.DevKey)
	if devKey == "" {
		return DevLoginResult{}, errors.New("dev_key is required")
	}
	if s == nil || s.repo == nil || s.sessions == nil {
		return DevLoginResult{}, errors.New("account service dependencies are nil")
	}

	user, err := s.repo.UpsertDevUser(ctx, DevUserInput{
		PublicID:     id.NewPublicID("usr"),
		WechatOpenID: "dev:" + devKey,
		Nickname:     strings.TrimSpace(input.Nickname),
	})
	if err != nil {
		return DevLoginResult{}, err
	}

	token := id.NewToken("dev")
	err = s.sessions.Set(ctx, token, auth.Session{
		UserID:       user.ID,
		UserPublicID: user.PublicID,
		Surface:      "user",
		CreatedAt:    time.Now().UTC(),
	}, s.sessionTTL)
	if err != nil {
		return DevLoginResult{}, err
	}

	return DevLoginResult{
		UserPublicID:     user.PublicID,
		Token:            token,
		OnboardingStatus: user.OnboardingStatus,
	}, nil
}
