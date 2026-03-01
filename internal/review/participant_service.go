package review

import (
	"context"
	"fmt"
)

type ParticipantService interface {
	AddParticipant(ctx context.Context, reviewID, userID, addedBy uint64, role, reason string) (*ReviewParticipant, error)
	RemoveParticipant(ctx context.Context, reviewID, participantID, userID uint64) error
	SelfSubscribe(ctx context.Context, reviewID, userID uint64) (*ReviewParticipant, error)
	SubmitDecision(ctx context.Context, reviewID, userID uint64, decision string) error
}

type participantService struct {
	repo   Repository
	logger interface{ Info(string, ...interface{}) }
}

func NewParticipantService(repo Repository) ParticipantService {
	return &participantService{repo: repo}
}

func (s *participantService) AddParticipant(ctx context.Context, reviewID, userID, addedBy uint64, role, reason string) (*ReviewParticipant, error) {
	review, err := s.repo.FindReviewByID(ctx, reviewID)
	if err != nil {
		return nil, err
	}

	if review.AuthorID != addedBy {
		return nil, fmt.Errorf("only the review author can add participants")
	}

	existing, _ := s.repo.FindParticipant(ctx, reviewID, userID)
	if existing != nil {
		return nil, fmt.Errorf("user is already a participant")
	}

	state := "pending"
	if role == "subscriber" {
		state = "watching"
	}

	p := &ReviewParticipant{
		ReviewID:    reviewID,
		UserID:      userID,
		Role:        role,
		State:       state,
		AddedBy:     addedBy,
		AddedReason: reason,
	}

	if err := s.repo.CreateParticipant(ctx, p); err != nil {
		return nil, err
	}

	return p, nil
}

func (s *participantService) RemoveParticipant(ctx context.Context, reviewID, participantID, userID uint64) error {
	review, err := s.repo.FindReviewByID(ctx, reviewID)
	if err != nil {
		return err
	}

	participant, err := s.repo.FindParticipantByID(ctx, participantID)
	if err != nil || participant == nil {
		return fmt.Errorf("participant not found")
	}

	if participant.Role == "author" {
		return fmt.Errorf("cannot remove the review author")
	}

	// Author can remove anyone; subscribers can remove themselves
	if review.AuthorID != userID && !(participant.UserID == userID && participant.Role == "subscriber") {
		return fmt.Errorf("forbidden")
	}

	return s.repo.DeleteParticipant(ctx, participantID)
}

func (s *participantService) SelfSubscribe(ctx context.Context, reviewID, userID uint64) (*ReviewParticipant, error) {
	existing, _ := s.repo.FindParticipant(ctx, reviewID, userID)
	if existing != nil {
		return existing, nil
	}

	p := &ReviewParticipant{
		ReviewID:    reviewID,
		UserID:      userID,
		Role:        "subscriber",
		State:       "watching",
		AddedBy:     userID,
		AddedReason: "manual",
	}

	if err := s.repo.CreateParticipant(ctx, p); err != nil {
		return nil, err
	}

	return p, nil
}

func (s *participantService) SubmitDecision(ctx context.Context, reviewID, userID uint64, decision string) error {
	review, err := s.repo.FindReviewByID(ctx, reviewID)
	if err != nil {
		return err
	}

	participant, err := s.repo.FindParticipant(ctx, reviewID, userID)
	if err != nil || participant == nil {
		return fmt.Errorf("user is not a participant")
	}

	// Validate decision permissions
	switch decision {
	case "comment":
		if participant.Role != "reviewer" && participant.Role != "blocking_reviewer" {
			return fmt.Errorf("only reviewers can submit decisions")
		}
		participant.State = "commented"

	case "changes_requested":
		if participant.Role != "reviewer" && participant.Role != "blocking_reviewer" {
			return fmt.Errorf("only reviewers can request changes")
		}
		participant.State = "changes_requested"

	case "accepted":
		if participant.Role != "reviewer" && participant.Role != "blocking_reviewer" {
			return fmt.Errorf("only reviewers can accept")
		}
		if review.AuthorID == userID {
			return fmt.Errorf("author cannot accept own review")
		}
		participant.State = "accepted"

	case "resolved":
		if err := checkCloseResolvePermission(review, userID); err != nil {
			return err
		}
		review.Status = "resolved"
		return s.repo.UpdateReview(ctx, review)

	case "closed":
		if err := checkCloseResolvePermission(review, userID); err != nil {
			return err
		}
		review.Status = "closed"
		return s.repo.UpdateReview(ctx, review)

	default:
		return fmt.Errorf("invalid decision: %s", decision)
	}

	if err := s.repo.UpdateParticipant(ctx, participant); err != nil {
		return err
	}

	// Recompute review status (unless decision is "comment" per E-003)
	if decision != "comment" {
		return RecomputeReviewStatus(ctx, s.repo, review)
	}

	return nil
}
