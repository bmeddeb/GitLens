package jobs

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"go.uber.org/zap"
)

type Service interface {
	Enqueue(ctx context.Context, jobType JobType, repoID uint64, forkID *uint64, requestedBy uint64) (*Job, bool, error)
	Get(ctx context.Context, id uint64) (*Job, error)
	Cancel(ctx context.Context, id uint64, userID uint64) error
	ListByUser(ctx context.Context, userID uint64, cursor uint64, limit int) ([]Job, error)
	ListByRepo(ctx context.Context, repoID uint64, cursor uint64, limit int) ([]Job, error)
	Complete(ctx context.Context, id uint64, summary JSONMap) error
	Fail(ctx context.Context, id uint64, errMsg string) error
	UpdateProgress(ctx context.Context, id uint64, pct uint8, msg string) error
}

type service struct {
	repo   Repository
	logger *zap.Logger
}

func NewService(repo Repository, logger *zap.Logger) Service {
	return &service{repo: repo, logger: logger}
}

func (s *service) Enqueue(ctx context.Context, jobType JobType, repoID uint64, forkID *uint64, requestedBy uint64) (*Job, bool, error) {
	dedupKey := computeDedupKey(jobType, repoID, forkID)

	existing, err := s.repo.FindByDedupKey(ctx, dedupKey)
	if err == nil && existing != nil {
		return existing, true, nil
	}

	job := &Job{
		Type:        jobType,
		Status:      JobStatusPending,
		RepoID:      repoID,
		ForkID:      forkID,
		DedupKey:    &dedupKey,
		RequestedBy: requestedBy,
		MaxRetries:  3,
	}

	if err := s.repo.Create(ctx, job); err != nil {
		// Race: another goroutine may have inserted the same dedup_key
		existing, findErr := s.repo.FindByDedupKey(ctx, dedupKey)
		if findErr == nil && existing != nil {
			return existing, true, nil
		}
		return nil, false, fmt.Errorf("enqueueing job: %w", err)
	}

	s.logger.Info("job enqueued",
		zap.Uint64("job_id", job.ID),
		zap.String("type", string(jobType)),
		zap.Uint64("repo_id", repoID),
	)

	return job, false, nil
}

func (s *service) Get(ctx context.Context, id uint64) (*Job, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *service) Cancel(ctx context.Context, id uint64, userID uint64) error {
	job, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if job.RequestedBy != userID {
		return fmt.Errorf("only the requester can cancel a job")
	}

	if job.Status != JobStatusPending {
		return fmt.Errorf("only pending jobs can be cancelled")
	}

	job.Status = JobStatusCancelled
	job.DedupKey = nil // Release dedup slot

	return s.repo.Update(ctx, job)
}

func (s *service) ListByUser(ctx context.Context, userID uint64, cursor uint64, limit int) ([]Job, error) {
	return s.repo.ListByUser(ctx, userID, cursor, limit)
}

func (s *service) ListByRepo(ctx context.Context, repoID uint64, cursor uint64, limit int) ([]Job, error) {
	return s.repo.ListByRepo(ctx, repoID, cursor, limit)
}

func (s *service) Complete(ctx context.Context, id uint64, summary JSONMap) error {
	job, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	job.Status = JobStatusCompleted
	job.ProgressPct = 100
	job.ResultSummary = summary
	job.DedupKey = nil // Release dedup slot

	return s.repo.Update(ctx, job)
}

func (s *service) Fail(ctx context.Context, id uint64, errMsg string) error {
	job, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	job.RetryCount++
	job.ErrorMessage = &errMsg

	if job.RetryCount < job.MaxRetries {
		job.Status = JobStatusPending
		job.StartedAt = nil
		s.logger.Info("job scheduled for retry",
			zap.Uint64("job_id", id),
			zap.Uint8("retry", job.RetryCount),
		)
	} else {
		job.Status = JobStatusFailed
		job.DedupKey = nil
	}

	return s.repo.Update(ctx, job)
}

func (s *service) UpdateProgress(ctx context.Context, id uint64, pct uint8, msg string) error {
	job, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	job.ProgressPct = pct
	job.ProgressMessage = &msg
	return s.repo.Update(ctx, job)
}

func computeDedupKey(jobType JobType, repoID uint64, forkID *uint64) string {
	var forkStr string
	if forkID != nil {
		forkStr = fmt.Sprintf("%d", *forkID)
	}
	data := fmt.Sprintf("%s|%d|%s", jobType, repoID, forkStr)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

var ErrForbidden = errors.New("forbidden")
