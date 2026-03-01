package review

import (
	"context"
)

// RecomputeReviewStatus derives the review status from participant states.
// Explicit statuses (draft, resolved, closed) are preserved.
func RecomputeReviewStatus(ctx context.Context, repo Repository, review *Review) error {
	// Explicit states are not derived
	if review.Status == "draft" || review.Status == "resolved" || review.Status == "closed" {
		return nil
	}

	participants, err := repo.ListParticipants(ctx, review.ID)
	if err != nil {
		return err
	}

	var (
		hasReviewers         bool
		anyChangesRequested  bool
		anyAccepted          bool
		allBlockingAccepted  bool = true
		hasBlockingReviewers bool
	)

	for _, p := range participants {
		if p.Role != "reviewer" && p.Role != "blocking_reviewer" {
			continue
		}
		hasReviewers = true

		if p.State == "changes_requested" {
			anyChangesRequested = true
		}
		if p.State == "accepted" {
			anyAccepted = true
		}

		if p.Role == "blocking_reviewer" {
			hasBlockingReviewers = true
			if p.State != "accepted" {
				allBlockingAccepted = false
			}
		}
	}

	newStatus := "open"

	if anyChangesRequested {
		newStatus = "changes_requested"
	} else if anyAccepted && (!hasBlockingReviewers || allBlockingAccepted) {
		newStatus = "accepted"
	}

	if !hasReviewers {
		newStatus = "open"
	}

	if review.Status != newStatus {
		review.Status = newStatus
		return repo.UpdateReview(ctx, review)
	}

	return nil
}
