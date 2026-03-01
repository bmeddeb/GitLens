package jobs

import (
	"context"
	"fmt"

	"github.com/user/gitlens-pro/internal/analysis"
	repomod "github.com/user/gitlens-pro/internal/repository"
	"go.uber.org/zap"
)

type ForkAnalysisWorker struct {
	forkAnalyzer analysis.ForkAnalyzer
	forkRepo     repomod.ForkRepository
	repoRepo     repomod.RepoRepository
	logger       *zap.Logger
}

func NewForkAnalysisWorker(
	forkAnalyzer analysis.ForkAnalyzer,
	forkRepo repomod.ForkRepository,
	repoRepo repomod.RepoRepository,
	logger *zap.Logger,
) *ForkAnalysisWorker {
	return &ForkAnalysisWorker{
		forkAnalyzer: forkAnalyzer,
		forkRepo:     forkRepo,
		repoRepo:     repoRepo,
		logger:       logger,
	}
}

func (w *ForkAnalysisWorker) Execute(ctx context.Context, job *Job) error {
	if job.ForkID == nil {
		return fmt.Errorf("fork_analysis job missing fork_id")
	}

	fork, err := w.forkRepo.FindByID(ctx, *job.ForkID)
	if err != nil || fork == nil {
		return fmt.Errorf("finding fork: %w", err)
	}

	sourceRepo, err := w.repoRepo.FindByID(ctx, job.RepoID)
	if err != nil {
		return fmt.Errorf("finding source repo: %w", err)
	}

	// Update status
	fork.AnalysisStatus = "analyzing"
	_ = w.forkRepo.Update(ctx, fork)

	if err := w.forkAnalyzer.AnalyzeFork(ctx, fork, sourceRepo); err != nil {
		fork.AnalysisStatus = "error"
		_ = w.forkRepo.Update(ctx, fork)
		return err
	}

	w.logger.Info("fork analysis complete",
		zap.Uint64("fork_id", fork.ID),
		zap.String("fork", fork.ForkFullName),
	)

	return nil
}
