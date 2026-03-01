package gitclient

import (
	"context"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/user/gitlens-pro/internal/config"
	"go.uber.org/zap"
)

type gitService struct {
	cfg    *config.AppConfig
	logger *zap.Logger
}

func NewGitService(cfg *config.AppConfig, logger *zap.Logger) GitService {
	return &gitService{cfg: cfg, logger: logger}
}

func (s *gitService) Clone(ctx context.Context, url, destPath string) error {
	s.logger.Info("cloning repository", zap.String("url", url), zap.String("dest", destPath))

	_, err := git.PlainCloneContext(ctx, destPath, false, &git.CloneOptions{
		URL:      url,
		Progress: nil,
	})
	if err != nil {
		return fmt.Errorf("cloning %s: %w", url, err)
	}

	return nil
}

func (s *gitService) Pull(ctx context.Context, repoPath string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("opening repo: %w", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	err = w.PullContext(ctx, &git.PullOptions{
		RemoteName: "origin",
		Force:      true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		// Fallback: fetch + reset
		s.logger.Warn("pull failed, trying fetch+reset", zap.Error(err))

		remote, err2 := repo.Remote("origin")
		if err2 != nil {
			return fmt.Errorf("getting remote: %w", err2)
		}

		if err2 = remote.FetchContext(ctx, &git.FetchOptions{Force: true}); err2 != nil && err2 != git.NoErrAlreadyUpToDate {
			return fmt.Errorf("fetching: %w", err2)
		}

		head, err2 := repo.Head()
		if err2 != nil {
			return fmt.Errorf("getting HEAD: %w", err2)
		}

		ref, err2 := repo.Reference(plumbing.NewRemoteReferenceName("origin", head.Name().Short()), true)
		if err2 != nil {
			return fmt.Errorf("resolving remote ref: %w", err2)
		}

		if err2 = w.Reset(&git.ResetOptions{
			Commit: ref.Hash(),
			Mode:   git.HardReset,
		}); err2 != nil {
			return fmt.Errorf("resetting: %w", err2)
		}
	}

	return nil
}
