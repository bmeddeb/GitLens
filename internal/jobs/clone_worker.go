package jobs

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/user/gitlens-pro/internal/analysis"
	"github.com/user/gitlens-pro/internal/config"
	"github.com/user/gitlens-pro/internal/gitclient"
	repomod "github.com/user/gitlens-pro/internal/repository"
	"go.uber.org/zap"
)

type CloneWorker struct {
	gitService     gitclient.GitService
	repoRepo       repomod.RepoRepository
	commitAnalyzer analysis.CommitAnalyzer
	cfg            *config.AppConfig
	logger         *zap.Logger
}

func NewCloneWorker(
	gitService gitclient.GitService,
	repoRepo repomod.RepoRepository,
	commitAnalyzer analysis.CommitAnalyzer,
	cfg *config.AppConfig,
	logger *zap.Logger,
) *CloneWorker {
	return &CloneWorker{
		gitService:     gitService,
		repoRepo:       repoRepo,
		commitAnalyzer: commitAnalyzer,
		cfg:            cfg,
		logger:         logger,
	}
}

func (w *CloneWorker) Execute(ctx context.Context, job *Job) error {
	repo, err := w.repoRepo.FindByID(ctx, job.RepoID)
	if err != nil {
		return fmt.Errorf("finding repo: %w", err)
	}

	// Update status to cloning
	repo.CloneStatus = "cloning"
	_ = w.repoRepo.Update(ctx, repo)

	// Clone to storage path
	destPath := filepath.Join(w.cfg.Storage.ReposPath, fmt.Sprintf("%d", repo.ID))
	localPath := destPath
	repo.LocalPath = &localPath

	if err := w.gitService.Clone(ctx, repo.CloneURL, destPath); err != nil {
		repo.CloneStatus = "error"
		_ = w.repoRepo.Update(ctx, repo)
		return fmt.Errorf("cloning: %w", err)
	}

	// Walk all commits and process
	commits, err := w.gitService.Log(ctx, destPath, gitclient.LogOptions{Limit: 100000})
	if err != nil {
		w.logger.Warn("failed to walk commits after clone", zap.Error(err))
	} else {
		if err := w.commitAnalyzer.ProcessNewCommits(ctx, repo.ID, commits); err != nil {
			w.logger.Warn("failed to process commits", zap.Error(err))
		}
	}

	// Mark ready
	repo.CloneStatus = "ready"
	if err := w.repoRepo.Update(ctx, repo); err != nil {
		return fmt.Errorf("updating repo status: %w", err)
	}

	w.logger.Info("clone complete",
		zap.Uint64("repo_id", repo.ID),
		zap.String("full_name", repo.FullName),
	)

	return nil
}
