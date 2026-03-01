package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

var ErrJobNotFound = errors.New("job not found")

type Repository interface {
	Create(ctx context.Context, job *Job) error
	FindByID(ctx context.Context, id uint64) (*Job, error)
	FindByDedupKey(ctx context.Context, dedupKey string) (*Job, error)
	Update(ctx context.Context, job *Job) error
	DequeueNext(ctx context.Context, jobType JobType) (*Job, error)
	ResetStaleRunning(ctx context.Context, staleTimeout time.Duration) (int64, error)
	ListByUser(ctx context.Context, userID uint64, cursor uint64, limit int) ([]Job, error)
	ListByRepo(ctx context.Context, repoID uint64, cursor uint64, limit int) ([]Job, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, job *Job) error {
	if err := r.db.WithContext(ctx).Create(job).Error; err != nil {
		return fmt.Errorf("creating job: %w", err)
	}
	return nil
}

func (r *repository) FindByID(ctx context.Context, id uint64) (*Job, error) {
	var job Job
	if err := r.db.WithContext(ctx).First(&job, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("finding job: %w", err)
	}
	return &job, nil
}

func (r *repository) FindByDedupKey(ctx context.Context, dedupKey string) (*Job, error) {
	var job Job
	if err := r.db.WithContext(ctx).Where("dedup_key = ?", dedupKey).First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("finding job by dedup_key: %w", err)
	}
	return &job, nil
}

func (r *repository) Update(ctx context.Context, job *Job) error {
	if err := r.db.WithContext(ctx).Save(job).Error; err != nil {
		return fmt.Errorf("updating job: %w", err)
	}
	return nil
}

func (r *repository) DequeueNext(ctx context.Context, jobType JobType) (*Job, error) {
	var job Job
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("type = ? AND status = ?", jobType, JobStatusPending).
			Order("created_at ASC").
			First(&job).Error; err != nil {
			return err
		}

		now := time.Now()
		job.Status = JobStatusRunning
		job.StartedAt = &now

		return tx.Save(&job).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("dequeuing job: %w", err)
	}
	return &job, nil
}

func (r *repository) ResetStaleRunning(ctx context.Context, staleTimeout time.Duration) (int64, error) {
	cutoff := time.Now().Add(-staleTimeout)
	result := r.db.WithContext(ctx).
		Model(&Job{}).
		Where("status = ? AND started_at < ?", JobStatusRunning, cutoff).
		Updates(map[string]interface{}{
			"status":     JobStatusPending,
			"started_at": nil,
		})
	if result.Error != nil {
		return 0, fmt.Errorf("resetting stale jobs: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (r *repository) ListByUser(ctx context.Context, userID uint64, cursor uint64, limit int) ([]Job, error) {
	var jobs []Job
	query := r.db.WithContext(ctx).Where("requested_by = ?", userID)
	if cursor > 0 {
		query = query.Where("id > ?", cursor)
	}
	if err := query.Order("id ASC").Limit(limit + 1).Find(&jobs).Error; err != nil {
		return nil, fmt.Errorf("listing user jobs: %w", err)
	}
	return jobs, nil
}

func (r *repository) ListByRepo(ctx context.Context, repoID uint64, cursor uint64, limit int) ([]Job, error) {
	var jobs []Job
	query := r.db.WithContext(ctx).Where("repo_id = ?", repoID)
	if cursor > 0 {
		query = query.Where("id > ?", cursor)
	}
	if err := query.Order("id ASC").Limit(limit + 1).Find(&jobs).Error; err != nil {
		return nil, fmt.Errorf("listing repo jobs: %w", err)
	}
	return jobs, nil
}
