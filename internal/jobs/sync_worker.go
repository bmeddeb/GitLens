package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/user/gitlens-pro/internal/analysis"
	"github.com/user/gitlens-pro/internal/gitclient"
	repomod "github.com/user/gitlens-pro/internal/repository"
	"go.uber.org/zap"
)

type SyncWorker struct {
	gitService     gitclient.GitService
	repoRepo       repomod.RepoRepository
	commitAnalyzer analysis.CommitAnalyzer
	logger         *zap.Logger
}

func NewSyncWorker(
	gitService gitclient.GitService,
	repoRepo repomod.RepoRepository,
	commitAnalyzer analysis.CommitAnalyzer,
	logger *zap.Logger,
) *SyncWorker {
	return &SyncWorker{
		gitService:     gitService,
		repoRepo:       repoRepo,
		commitAnalyzer: commitAnalyzer,
		logger:         logger,
	}
}

func (w *SyncWorker) Execute(ctx context.Context, job *Job) error {
	repo, err := w.repoRepo.FindByID(ctx, job.RepoID)
	if err != nil {
		return fmt.Errorf("finding repo: %w", err)
	}

	if repo.LocalPath == nil || *repo.LocalPath == "" {
		return fmt.Errorf("repo has no local path (not cloned)")
	}

	repoPath := *repo.LocalPath

	// Pull latest
	if err := w.gitService.Pull(ctx, repoPath); err != nil {
		return fmt.Errorf("pulling: %w", err)
	}

	// Walk new commits since last sync
	var commits []gitclient.CommitInfo
	if repo.LastSyncedAt != nil {
		since := *repo.LastSyncedAt
		commits, err = w.gitService.Log(ctx, repoPath, gitclient.LogOptions{
			Since: &since,
			Limit: 10000,
		})
	} else {
		commits, err = w.gitService.Log(ctx, repoPath, gitclient.LogOptions{Limit: 100000})
	}
	if err != nil {
		return fmt.Errorf("walking commits: %w", err)
	}

	// Process new commits (idempotent)
	if len(commits) > 0 {
		if err := w.commitAnalyzer.ProcessNewCommits(ctx, repo.ID, commits); err != nil {
			return fmt.Errorf("processing commits: %w", err)
		}
	}

	// Update last synced
	now := time.Now()
	repo.LastSyncedAt = &now
	if err := w.repoRepo.Update(ctx, repo); err != nil {
		return fmt.Errorf("updating repo sync time: %w", err)
	}

	w.logger.Info("sync complete",
		zap.Uint64("repo_id", repo.ID),
		zap.Int("new_commits", len(commits)),
	)

	return nil
}
