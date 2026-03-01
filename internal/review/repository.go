package review

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var (
	ErrReviewNotFound   = errors.New("review not found")
	ErrRevisionNotFound = errors.New("revision not found")
)

type Repository interface {
	CreateReview(ctx context.Context, r *Review) error
	FindReviewByID(ctx context.Context, id uint64) (*Review, error)
	UpdateReview(ctx context.Context, r *Review) error
	ListReviews(ctx context.Context, repoID uint64, cursor uint64, limit int) ([]Review, error)

	CreateRevision(ctx context.Context, rev *ReviewRevision) error
	FindRevisionByID(ctx context.Context, id uint64) (*ReviewRevision, error)
	CountRevisions(ctx context.Context, reviewID uint64) (int64, error)
	LatestRevision(ctx context.Context, reviewID uint64) (*ReviewRevision, error)

	CreateParticipant(ctx context.Context, p *ReviewParticipant) error
	FindParticipant(ctx context.Context, reviewID, userID uint64) (*ReviewParticipant, error)
	FindParticipantByID(ctx context.Context, id uint64) (*ReviewParticipant, error)
	UpdateParticipant(ctx context.Context, p *ReviewParticipant) error
	DeleteParticipant(ctx context.Context, id uint64) error
	ListParticipants(ctx context.Context, reviewID uint64) ([]ReviewParticipant, error)
	ResetParticipantStates(ctx context.Context, reviewID uint64) error

	CreateComment(ctx context.Context, c *ReviewComment) error
	UpdateComment(ctx context.Context, c *ReviewComment) error
	FindCommentByID(ctx context.Context, id uint64) (*ReviewComment, error)
	ListComments(ctx context.Context, reviewID uint64, visibility string, cursor uint64, limit int) ([]ReviewComment, error)
	ListDraftsByUser(ctx context.Context, reviewID, userID uint64) ([]ReviewComment, error)
	ListInlineCommentsByRevision(ctx context.Context, reviewID, revisionID uint64) ([]ReviewComment, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateReview(ctx context.Context, rev *Review) error {
	return r.db.WithContext(ctx).Create(rev).Error
}

func (r *repository) FindReviewByID(ctx context.Context, id uint64) (*Review, error) {
	var rev Review
	if err := r.db.WithContext(ctx).Preload("LatestRevision").Preload("Participants").First(&rev, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReviewNotFound
		}
		return nil, err
	}
	return &rev, nil
}

func (r *repository) UpdateReview(ctx context.Context, rev *Review) error {
	return r.db.WithContext(ctx).Save(rev).Error
}

func (r *repository) ListReviews(ctx context.Context, repoID uint64, cursor uint64, limit int) ([]Review, error) {
	query := r.db.WithContext(ctx).Where("repo_id = ?", repoID)
	if cursor > 0 {
		query = query.Where("id > ?", cursor)
	}
	var reviews []Review
	if err := query.Order("updated_at DESC").Limit(limit + 1).Find(&reviews).Error; err != nil {
		return nil, err
	}
	return reviews, nil
}

func (r *repository) CreateRevision(ctx context.Context, rev *ReviewRevision) error {
	return r.db.WithContext(ctx).Create(rev).Error
}

func (r *repository) FindRevisionByID(ctx context.Context, id uint64) (*ReviewRevision, error) {
	var rev ReviewRevision
	if err := r.db.WithContext(ctx).First(&rev, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRevisionNotFound
		}
		return nil, err
	}
	return &rev, nil
}

func (r *repository) CountRevisions(ctx context.Context, reviewID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&ReviewRevision{}).Where("review_id = ?", reviewID).Count(&count).Error
	return count, err
}

func (r *repository) LatestRevision(ctx context.Context, reviewID uint64) (*ReviewRevision, error) {
	var rev ReviewRevision
	if err := r.db.WithContext(ctx).Where("review_id = ?", reviewID).Order("revision_number DESC").First(&rev).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rev, nil
}

func (r *repository) CreateParticipant(ctx context.Context, p *ReviewParticipant) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *repository) FindParticipant(ctx context.Context, reviewID, userID uint64) (*ReviewParticipant, error) {
	var p ReviewParticipant
	if err := r.db.WithContext(ctx).Where("review_id = ? AND user_id = ?", reviewID, userID).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *repository) FindParticipantByID(ctx context.Context, id uint64) (*ReviewParticipant, error) {
	var p ReviewParticipant
	if err := r.db.WithContext(ctx).First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *repository) UpdateParticipant(ctx context.Context, p *ReviewParticipant) error {
	return r.db.WithContext(ctx).Save(p).Error
}

func (r *repository) DeleteParticipant(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&ReviewParticipant{}, id).Error
}

func (r *repository) ListParticipants(ctx context.Context, reviewID uint64) ([]ReviewParticipant, error) {
	var participants []ReviewParticipant
	if err := r.db.WithContext(ctx).Where("review_id = ?", reviewID).Find(&participants).Error; err != nil {
		return nil, err
	}
	return participants, nil
}

func (r *repository) ResetParticipantStates(ctx context.Context, reviewID uint64) error {
	return r.db.WithContext(ctx).
		Model(&ReviewParticipant{}).
		Where("review_id = ? AND role IN ? AND state != ?", reviewID, []string{"reviewer", "blocking_reviewer"}, "pending").
		Updates(map[string]interface{}{"state": "pending"}).Error
}

func (r *repository) CreateComment(ctx context.Context, c *ReviewComment) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *repository) UpdateComment(ctx context.Context, c *ReviewComment) error {
	return r.db.WithContext(ctx).Save(c).Error
}

func (r *repository) FindCommentByID(ctx context.Context, id uint64) (*ReviewComment, error) {
	var c ReviewComment
	if err := r.db.WithContext(ctx).First(&c, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (r *repository) ListComments(ctx context.Context, reviewID uint64, visibility string, cursor uint64, limit int) ([]ReviewComment, error) {
	query := r.db.WithContext(ctx).Where("review_id = ?", reviewID)
	if visibility != "" {
		query = query.Where("visibility = ?", visibility)
	}
	if cursor > 0 {
		query = query.Where("id > ?", cursor)
	}
	var comments []ReviewComment
	if err := query.Order("created_at ASC").Limit(limit + 1).Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
}

func (r *repository) ListDraftsByUser(ctx context.Context, reviewID, userID uint64) ([]ReviewComment, error) {
	var comments []ReviewComment
	if err := r.db.WithContext(ctx).
		Where("review_id = ? AND author_id = ? AND visibility = ?", reviewID, userID, "draft").
		Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
}

func (r *repository) ListInlineCommentsByRevision(ctx context.Context, reviewID, revisionID uint64) ([]ReviewComment, error) {
	var comments []ReviewComment
	if err := r.db.WithContext(ctx).
		Where("review_id = ? AND revision_id = ? AND file_path IS NOT NULL AND visibility = ?", reviewID, revisionID, "published").
		Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
}

// Ensure interface compliance
var _ Repository = (*repository)(nil)

// Unused import guard
var _ = fmt.Sprintf
