package review

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type CommentService interface {
	CreateDraft(ctx context.Context, reviewID, userID uint64, req CreateCommentRequest) (*ReviewComment, error)
	PublishDrafts(ctx context.Context, reviewID, userID uint64, decision string) ([]ReviewComment, error)
	ListComments(ctx context.Context, reviewID uint64, visibility string, userID uint64, cursor uint64, limit int) ([]ReviewComment, error)
	ResolveComment(ctx context.Context, commentID, userID uint64) error
	ReopenComment(ctx context.Context, commentID, userID uint64) error
}

type CreateCommentRequest struct {
	RevisionID      uint64  `json:"revision_id"`
	ParentCommentID *uint64 `json:"parent_comment_id,omitempty"`
	FilePath        *string `json:"file_path,omitempty"`
	Side            *string `json:"side,omitempty"`
	StartLine       *uint   `json:"start_line,omitempty"`
	EndLine         *uint   `json:"end_line,omitempty"`
	Body            string  `json:"body"`
}

type commentService struct {
	repo Repository
}

func NewCommentService(repo Repository) CommentService {
	return &commentService{repo: repo}
}

func (s *commentService) CreateDraft(ctx context.Context, reviewID, userID uint64, req CreateCommentRequest) (*ReviewComment, error) {
	comment := &ReviewComment{
		ReviewID:   reviewID,
		RevisionID: req.RevisionID,
		AuthorID:   userID,
		Body:       req.Body,
		Visibility: "draft",
	}
	if req.ParentCommentID != nil {
		comment.ParentCommentID = req.ParentCommentID
	}
	if req.FilePath != nil {
		comment.FilePath = req.FilePath
	}
	if req.Side != nil {
		comment.Side = req.Side
	}
	if req.StartLine != nil {
		comment.StartLine = req.StartLine
	}
	if req.EndLine != nil {
		comment.EndLine = req.EndLine
	}

	if err := s.repo.CreateComment(ctx, comment); err != nil {
		return nil, fmt.Errorf("creating draft: %w", err)
	}

	return comment, nil
}

func (s *commentService) PublishDrafts(ctx context.Context, reviewID, userID uint64, decision string) ([]ReviewComment, error) {
	drafts, err := s.repo.ListDraftsByUser(ctx, reviewID, userID)
	if err != nil {
		return nil, err
	}

	if len(drafts) == 0 && decision == "" {
		return nil, fmt.Errorf("no drafts to publish")
	}

	batchID := uuid.New().String()
	now := time.Now()

	for i := range drafts {
		drafts[i].Visibility = "published"
		drafts[i].PublishBatchID = &batchID
		drafts[i].PublishedAt = &now

		// Compute fingerprint for inline comments
		if drafts[i].FilePath != nil && drafts[i].StartLine != nil {
			// Fingerprint would be computed from the diff context
			// Placeholder: stored as nil until diff context is available
		}

		if err := s.repo.UpdateComment(ctx, &drafts[i]); err != nil {
			return nil, fmt.Errorf("publishing comment %d: %w", drafts[i].ID, err)
		}
	}

	return drafts, nil
}

func (s *commentService) ListComments(ctx context.Context, reviewID uint64, visibility string, userID uint64, cursor uint64, limit int) ([]ReviewComment, error) {
	if visibility == "draft" {
		return s.repo.ListDraftsByUser(ctx, reviewID, userID)
	}
	return s.repo.ListComments(ctx, reviewID, visibility, cursor, limit)
}

func (s *commentService) ResolveComment(ctx context.Context, commentID, userID uint64) error {
	comment, err := s.repo.FindCommentByID(ctx, commentID)
	if err != nil || comment == nil {
		return fmt.Errorf("comment not found")
	}

	review, err := s.repo.FindReviewByID(ctx, comment.ReviewID)
	if err != nil {
		return err
	}

	// Permission: comment author, review author, or blocking reviewer
	if !canResolveComment(review, comment, userID) {
		return fmt.Errorf("forbidden")
	}

	now := time.Now()
	comment.IsResolved = true
	comment.ResolvedAt = &now
	return s.repo.UpdateComment(ctx, comment)
}

func (s *commentService) ReopenComment(ctx context.Context, commentID, userID uint64) error {
	comment, err := s.repo.FindCommentByID(ctx, commentID)
	if err != nil || comment == nil {
		return fmt.Errorf("comment not found")
	}

	review, err := s.repo.FindReviewByID(ctx, comment.ReviewID)
	if err != nil {
		return err
	}

	if !canResolveComment(review, comment, userID) {
		return fmt.Errorf("forbidden")
	}

	comment.IsResolved = false
	comment.ResolvedAt = nil
	return s.repo.UpdateComment(ctx, comment)
}

func canResolveComment(review *Review, comment *ReviewComment, userID uint64) bool {
	if comment.AuthorID == userID || review.AuthorID == userID {
		return true
	}
	for _, p := range review.Participants {
		if p.UserID == userID && p.Role == "blocking_reviewer" {
			return true
		}
	}
	return false
}
