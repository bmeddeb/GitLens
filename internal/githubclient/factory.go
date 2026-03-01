package githubclient

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/go-github/v60/github"
	"github.com/user/gitlens-pro/internal/auth"
	"github.com/user/gitlens-pro/internal/config"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type GitHubClientFactory interface {
	NewClient(ctx context.Context, encryptedToken []byte) (*github.Client, error)
	GetRateLimit(ctx context.Context, client *github.Client) (*github.RateLimits, error)
}

type factory struct {
	encryptor  *auth.Encryptor
	cfg        *config.AppConfig
	logger     *zap.Logger
	rateLimits sync.Map // token hash -> *RateLimitState
}

type RateLimitState struct {
	Remaining int
	ResetAt   int64
}

func NewGitHubClientFactory(
	encryptor *auth.Encryptor,
	cfg *config.AppConfig,
	logger *zap.Logger,
) GitHubClientFactory {
	return &factory{
		encryptor: encryptor,
		cfg:       cfg,
		logger:    logger,
	}
}

func (f *factory) NewClient(ctx context.Context, encryptedToken []byte) (*github.Client, error) {
	token, err := f.encryptor.Decrypt(encryptedToken)
	if err != nil {
		return nil, fmt.Errorf("decrypting token: %w", err)
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: string(token)})
	tc := oauth2.NewClient(ctx, ts)

	return github.NewClient(tc), nil
}

func (f *factory) GetRateLimit(ctx context.Context, client *github.Client) (*github.RateLimits, error) {
	limits, _, err := client.RateLimit.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting rate limit: %w", err)
	}
	return limits, nil
}
