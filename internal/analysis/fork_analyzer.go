package analysis

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/user/gitlens-pro/internal/config"
	"github.com/user/gitlens-pro/internal/gitclient"
	"github.com/user/gitlens-pro/internal/repository"
	"go.uber.org/zap"
)

type ForkAnalyzer interface {
	DiscoverForks(ctx context.Context, repoID uint64) error
	AnalyzeFork(ctx context.Context, fork *repository.Fork, sourceRepo *repository.Repository) error
}

type forkAnalyzer struct {
	gitService gitclient.GitService
	forkRepo   repository.ForkRepository
	classifier *Classifier
	cherryPick *CherryPickDetector
	rebase     *RebaseDetector
	similarity *SimilarityDetector
	cfg        *config.AppConfig
	logger     *zap.Logger
}

func NewForkAnalyzer(
	gitService gitclient.GitService,
	forkRepo repository.ForkRepository,
	cfg *config.AppConfig,
	logger *zap.Logger,
) ForkAnalyzer {
	return &forkAnalyzer{
		gitService: gitService,
		forkRepo:   forkRepo,
		classifier: NewClassifier(),
		cherryPick: NewCherryPickDetector(),
		rebase:     NewRebaseDetector(),
		similarity: NewSimilarityDetector(),
		cfg:        cfg,
		logger:     logger,
	}
}

func (a *forkAnalyzer) DiscoverForks(ctx context.Context, repoID uint64) error {
	// Fork discovery is done via GitHub API in the repository service
	// This method is for triggering analysis of already-discovered forks
	return nil
}

func (a *forkAnalyzer) AnalyzeFork(ctx context.Context, fork *repository.Fork, sourceRepo *repository.Repository) error {
	if fork.LocalPath == nil || *fork.LocalPath == "" {
		// Clone fork
		destPath := filepath.Join(a.cfg.Storage.ForksPath, fmt.Sprintf("%d", fork.ID))
		if fork.CloneURL != nil {
			if err := a.gitService.Clone(ctx, *fork.CloneURL, destPath); err != nil {
				return fmt.Errorf("cloning fork: %w", err)
			}
			fork.LocalPath = &destPath
		}
	}

	if sourceRepo.LocalPath == nil {
		return fmt.Errorf("source repo not cloned")
	}

	// Find merge base
	mergeBase, err := a.gitService.MergeBase(ctx, *sourceRepo.LocalPath, "HEAD", "HEAD")
	if err != nil {
		a.logger.Warn("merge-base failed", zap.Error(err))
	} else {
		fork.MergeBaseSHA = &mergeBase
	}

	// Walk fork-specific commits
	if fork.LocalPath != nil {
		forkCommits, err := a.gitService.Log(ctx, *fork.LocalPath, gitclient.LogOptions{Limit: 10000})
		if err != nil {
			return fmt.Errorf("getting fork commits: %w", err)
		}

		sourceCommits, err := a.gitService.Log(ctx, *sourceRepo.LocalPath, gitclient.LogOptions{Limit: 10000})
		if err != nil {
			return fmt.Errorf("getting source commits: %w", err)
		}

		// Build source commit set for cherry-pick/rebase detection
		sourceSet := make(map[string]gitclient.CommitInfo)
		for _, c := range sourceCommits {
			sourceSet[c.SHA] = c
		}

		// Count ahead/behind
		sourceSHAs := make(map[string]bool)
		for _, c := range sourceCommits {
			sourceSHAs[c.SHA] = true
		}

		var ahead, behind uint
		forkSHAs := make(map[string]bool)
		for _, c := range forkCommits {
			forkSHAs[c.SHA] = true
			if !sourceSHAs[c.SHA] {
				ahead++
			}
		}
		for _, c := range sourceCommits {
			if !forkSHAs[c.SHA] {
				behind++
			}
		}
		fork.AheadBy = ahead
		fork.BehindBy = behind

		// Classify each unique fork commit
		var changes []repository.ForkChange
		for _, c := range forkCommits {
			if sourceSHAs[c.SHA] {
				continue // Common commit
			}

			change := repository.ForkChange{
				ForkID:    fork.ID,
				CommitSHA: c.SHA,
			}

			// Cherry-pick detection
			if matched, upstreamSHA := a.cherryPick.Detect(c, sourceCommits); matched {
				change.IsCherryPick = true
				change.UpstreamSHA = &upstreamSHA
			}

			// Rebase detection
			if !change.IsCherryPick {
				if matched, upstreamSHA := a.rebase.Detect(c, sourceCommits); matched {
					change.IsRebase = true
					change.UpstreamSHA = &upstreamSHA
				}
			}

			// Classification
			changeType, confidence := a.classifier.Classify(c)
			change.ChangeType = changeType
			change.Confidence = confidence

			// File path (use first changed file as representative)
			if len(c.ParentSHAs) > 0 && fork.LocalPath != nil {
				diff, err := a.gitService.Diff(ctx, *fork.LocalPath, c.ParentSHAs[0], c.SHA)
				if err == nil && len(diff.Files) > 0 {
					change.FilePath = &diff.Files[0].NewPath
				}
			}

			changes = append(changes, change)
		}

		if len(changes) > 0 {
			if err := a.forkRepo.CreateForkChangeBatch(ctx, changes); err != nil {
				return fmt.Errorf("storing fork changes: %w", err)
			}
		}
	}

	fork.AnalysisStatus = "complete"
	if err := a.forkRepo.Update(ctx, fork); err != nil {
		return fmt.Errorf("updating fork: %w", err)
	}

	return nil
}
