package review

import (
	"context"
	"fmt"
	"time"

	repomod "github.com/user/gitlens-pro/internal/repository"
	"go.uber.org/zap"
)

type Service interface {
	CreateReview(ctx context.Context, req CreateReviewRequest) (*Review, error)
	GetReview(ctx context.Context, reviewID uint64) (*Review, error)
	ListReviews(ctx context.Context, repoID uint64, cursor uint64, limit int) ([]Review, error)
	AddRevision(ctx context.Context, reviewID uint64, userID uint64, req AddRevisionRequest) (*ReviewRevision, error)
	Close(ctx context.Context, reviewID uint64, userID uint64) error
	Resolve(ctx context.Context, reviewID uint64, userID uint64) error
	Reopen(ctx context.Context, reviewID uint64, userID uint64) error
}

type CreateReviewRequest struct {
	RepoID        uint64 `json:"repo_id"`
	Kind          string `json:"kind"`
	Title         string `json:"title"`
	Summary       string `json:"summary,omitempty"`
	OriginType    string `json:"origin_type"`
	OriginPayload string `json:"origin_payload,omitempty"`
	AuthorID      uint64 `json:"-"`
	Draft         bool   `json:"draft"`
	BaseCommitSHA string `json:"base_commit_sha"`
	HeadCommitSHA string `json:"head_commit_sha"`
}

type AddRevisionRequest struct {
	BaseRef       string `json:"base_ref"`
	HeadRef       string `json:"head_ref"`
	BaseCommitSHA string `json:"base_commit_sha"`
	HeadCommitSHA string `json:"head_commit_sha"`
}

type service struct {
	repo        Repository
	repoService repomod.RepoService
	reanchor    *Reanchorer
	logger      *zap.Logger
}

func NewService(repo Repository, repoService repomod.RepoService, logger *zap.Logger) Service {
	return &service{
		repo:        repo,
		repoService: repoService,
		reanchor:    NewReanchorer(repo, logger),
		logger:      logger,
	}
}

func (s *service) CreateReview(ctx context.Context, req CreateReviewRequest) (*Review, error) {
	status := "open"
	if req.Draft {
		status = "draft"
	}

	review := &Review{
		RepoID:     req.RepoID,
		Kind:       req.Kind,
		Status:     status,
		AuthorID:   req.AuthorID,
		Title:      req.Title,
		OriginType: req.OriginType,
	}
	if req.Summary != "" {
		review.Summary = &req.Summary
	}
	if req.OriginPayload != "" {
		review.OriginPayload = &req.OriginPayload
	}

	if err := s.repo.CreateReview(ctx, review); err != nil {
		return nil, fmt.Errorf("creating review: %w", err)
	}

	// Create initial revision
	rev := &ReviewRevision{
		ReviewID:       review.ID,
		RevisionNumber: 1,
		BaseCommitSHA:  req.BaseCommitSHA,
		HeadCommitSHA:  req.HeadCommitSHA,
		CreatedBy:      req.AuthorID,
	}
	if err := s.repo.CreateRevision(ctx, rev); err != nil {
		return nil, fmt.Errorf("creating initial revision: %w", err)
	}

	review.LatestRevisionID = &rev.ID
	if err := s.repo.UpdateReview(ctx, review); err != nil {
		return nil, fmt.Errorf("updating latest_revision_id: %w", err)
	}

	// Add author as participant
	author := &ReviewParticipant{
		ReviewID:    review.ID,
		UserID:      req.AuthorID,
		Role:        "author",
		State:       "pending",
		AddedBy:     req.AuthorID,
		AddedReason: "manual",
	}
	if err := s.repo.CreateParticipant(ctx, author); err != nil {
		return nil, fmt.Errorf("adding author participant: %w", err)
	}

	return review, nil
}

func (s *service) GetReview(ctx context.Context, reviewID uint64) (*Review, error) {
	return s.repo.FindReviewByID(ctx, reviewID)
}

func (s *service) ListReviews(ctx context.Context, repoID uint64, cursor uint64, limit int) ([]Review, error) {
	return s.repo.ListReviews(ctx, repoID, cursor, limit)
}

func (s *service) AddRevision(ctx context.Context, reviewID uint64, userID uint64, req AddRevisionRequest) (*ReviewRevision, error) {
	review, err := s.repo.FindReviewByID(ctx, reviewID)
	if err != nil {
		return nil, err
	}

	if review.AuthorID != userID {
		return nil, fmt.Errorf("only the review author can add revisions")
	}

	count, err := s.repo.CountRevisions(ctx, reviewID)
	if err != nil {
		return nil, err
	}

	if count >= 50 {
		return nil, fmt.Errorf("maximum 50 revisions per review")
	}

	previousRevision, _ := s.repo.LatestRevision(ctx, reviewID)

	rev := &ReviewRevision{
		ReviewID:       reviewID,
		RevisionNumber: uint(count + 1),
		BaseCommitSHA:  req.BaseCommitSHA,
		HeadCommitSHA:  req.HeadCommitSHA,
		CreatedBy:      userID,
	}
	if req.BaseRef != "" {
		rev.BaseRef = &req.BaseRef
	}
	if req.HeadRef != "" {
		rev.HeadRef = &req.HeadRef
	}

	if err := s.repo.CreateRevision(ctx, rev); err != nil {
		return nil, err
	}

	// Update latest revision
	review.LatestRevisionID = &rev.ID
	if err := s.repo.UpdateReview(ctx, review); err != nil {
		return nil, err
	}

	// Reset participant states
	if err := s.repo.ResetParticipantStates(ctx, reviewID); err != nil {
		return nil, err
	}

	// Recompute status
	if err := RecomputeReviewStatus(ctx, s.repo, review); err != nil {
		s.logger.Warn("failed to recompute status after revision", zap.Error(err))
	}

	// Re-anchor comments from previous revision
	if previousRevision != nil {
		if err := s.reanchor.Reanchor(ctx, reviewID, previousRevision.ID, rev.ID); err != nil {
			s.logger.Warn("re-anchoring failed", zap.Error(err))
		}
	}

	return rev, nil
}

func (s *service) Close(ctx context.Context, reviewID uint64, userID uint64) error {
	review, err := s.repo.FindReviewByID(ctx, reviewID)
	if err != nil {
		return err
	}

	if err := checkCloseResolvePermission(review, userID); err != nil {
		return err
	}

	now := time.Now()
	review.Status = "closed"
	review.ClosedAt = &now
	return s.repo.UpdateReview(ctx, review)
}

func (s *service) Resolve(ctx context.Context, reviewID uint64, userID uint64) error {
	review, err := s.repo.FindReviewByID(ctx, reviewID)
	if err != nil {
		return err
	}

	if err := checkCloseResolvePermission(review, userID); err != nil {
		return err
	}

	review.Status = "resolved"
	return s.repo.UpdateReview(ctx, review)
}

func (s *service) Reopen(ctx context.Context, reviewID uint64, userID uint64) error {
	review, err := s.repo.FindReviewByID(ctx, reviewID)
	if err != nil {
		return err
	}

	if review.Status != "resolved" && review.Status != "closed" {
		return fmt.Errorf("can only reopen resolved or closed reviews")
	}

	if err := checkCloseResolvePermission(review, userID); err != nil {
		return err
	}

	review.Status = "open"
	review.ClosedAt = nil
	return s.repo.UpdateReview(ctx, review)
}

func checkCloseResolvePermission(review *Review, userID uint64) error {
	if review.AuthorID == userID {
		return nil
	}
	for _, p := range review.Participants {
		if p.UserID == userID && p.Role == "blocking_reviewer" {
			return nil
		}
	}
	return fmt.Errorf("only the author or a blocking reviewer can perform this action")
}
