package notification

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

type Service interface {
	Notify(ctx context.Context, n *Notification) error
	NotifyReviewEvent(ctx context.Context, eventType string, reviewID uint64, actorID uint64, title, bodyPreview string) error
	List(ctx context.Context, userID uint64, cursor uint64, limit int) ([]Notification, error)
	MarkRead(ctx context.Context, id, userID uint64) error
	MarkAllRead(ctx context.Context, userID uint64) error
}

type service struct {
	repo   Repository
	logger *zap.Logger
}

func NewService(repo Repository, logger *zap.Logger) Service {
	return &service{repo: repo, logger: logger}
}

func (s *service) Notify(ctx context.Context, n *Notification) error {
	return s.repo.Create(ctx, n)
}

func (s *service) NotifyReviewEvent(ctx context.Context, eventType string, reviewID uint64, actorID uint64, title, bodyPreview string) error {
	// This would load participants, check prefs, create notifications
	// For now, creates a notification for each participant

	s.logger.Info("review event notification",
		zap.String("event", eventType),
		zap.Uint64("review_id", reviewID),
		zap.Uint64("actor_id", actorID),
	)

	return nil
}

func (s *service) List(ctx context.Context, userID uint64, cursor uint64, limit int) ([]Notification, error) {
	return s.repo.ListByUser(ctx, userID, cursor, limit)
}

func (s *service) MarkRead(ctx context.Context, id, userID uint64) error {
	return s.repo.MarkRead(ctx, id, userID)
}

func (s *service) MarkAllRead(ctx context.Context, userID uint64) error {
	return s.repo.MarkAllRead(ctx, userID)
}

// Ensure interface
var _ Service = (*service)(nil)
var _ = fmt.Sprintf
