package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
)

type Service interface {
	FindOrCreateByGitHub(ctx context.Context, info *GitHubUserInfo) (*User, error)
	GetByID(ctx context.Context, id uint64) (*User, error)
}

type GitHubUserInfo struct {
	GitHubID    uint64
	Username    string
	DisplayName string
	Email       string
	AvatarURL   string
	AccessToken []byte // Already encrypted
}

type service struct {
	repo   Repository
	logger *zap.Logger
}

func NewService(repo Repository, logger *zap.Logger) Service {
	return &service{repo: repo, logger: logger}
}

func (s *service) FindOrCreateByGitHub(ctx context.Context, info *GitHubUserInfo) (*User, error) {
	u, err := s.repo.FindByGitHubID(ctx, info.GitHubID)
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	now := time.Now()

	if u != nil {
		u.Username = info.Username
		u.AccessToken = info.AccessToken
		u.LastLoginAt = &now

		if info.DisplayName != "" {
			u.DisplayName = &info.DisplayName
		}
		if info.Email != "" {
			u.Email = &info.Email
		}
		if info.AvatarURL != "" {
			u.AvatarURL = &info.AvatarURL
		}

		if err := s.repo.Update(ctx, u); err != nil {
			return nil, fmt.Errorf("updating user: %w", err)
		}

		s.logger.Info("user updated on login",
			zap.Uint64("user_id", u.ID),
			zap.String("username", u.Username),
		)
		return u, nil
	}

	u = &User{
		GitHubID:    info.GitHubID,
		Username:    info.Username,
		AccessToken: info.AccessToken,
		Role:        "user",
		LastLoginAt: &now,
	}

	if info.DisplayName != "" {
		u.DisplayName = &info.DisplayName
	}
	if info.Email != "" {
		u.Email = &info.Email
	}
	if info.AvatarURL != "" {
		u.AvatarURL = &info.AvatarURL
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	s.logger.Info("new user created",
		zap.Uint64("user_id", u.ID),
		zap.String("username", u.Username),
	)
	return u, nil
}

func (s *service) GetByID(ctx context.Context, id uint64) (*User, error) {
	return s.repo.FindByID(ctx, id)
}
