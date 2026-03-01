package review

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/user/gitlens-pro/internal/analysis"
	"go.uber.org/zap"
)

const contextRadius = 3

type Reanchorer struct {
	repo   Repository
	logger *zap.Logger
}

func NewReanchorer(repo Repository, logger *zap.Logger) *Reanchorer {
	return &Reanchorer{repo: repo, logger: logger}
}

// Reanchor runs the ADR-004 three-phase cascade on comments from oldRevisionID,
// mapping them to newRevisionID.
func (r *Reanchorer) Reanchor(ctx context.Context, reviewID, oldRevisionID, newRevisionID uint64) error {
	comments, err := r.repo.ListInlineCommentsByRevision(ctx, reviewID, oldRevisionID)
	if err != nil {
		return fmt.Errorf("listing comments for re-anchor: %w", err)
	}

	for _, comment := range comments {
		if comment.FilePath == nil || comment.ContextFingerprint == nil {
			continue
		}

		// Phase 1: Fingerprint match
		matched := r.tryFingerprintMatch(ctx, &comment, newRevisionID)
		if matched {
			comment.AnchorMethod = "fingerprint"
			_ = r.repo.UpdateComment(ctx, &comment)
			continue
		}

		// Phase 2: Line-offset with Levenshtein confidence
		matched = r.tryOffsetMatch(ctx, &comment, newRevisionID)
		if matched {
			comment.AnchorMethod = "offset"
			_ = r.repo.UpdateComment(ctx, &comment)
			continue
		}

		// Phase 3: Orphan
		now := time.Now()
		comment.AnchorMethod = "orphaned"
		comment.OrphanedAt = &now
		_ = r.repo.UpdateComment(ctx, &comment)

		r.logger.Info("comment orphaned",
			zap.Uint64("comment_id", comment.ID),
			zap.Uint64("review_id", reviewID),
		)
	}

	return nil
}

func (r *Reanchorer) tryFingerprintMatch(ctx context.Context, comment *ReviewComment, newRevisionID uint64) bool {
	// In a full implementation, we would:
	// 1. Read the new revision's diff for the same file_path
	// 2. Slide a window across the diff lines
	// 3. Compute SHA-256 of each window
	// 4. Compare with comment.ContextFingerprint
	// For now, this is a placeholder that returns false (triggering offset/orphan)
	return false
}

func (r *Reanchorer) tryOffsetMatch(ctx context.Context, comment *ReviewComment, newRevisionID uint64) bool {
	// In a full implementation, we would:
	// 1. Compute diff between old and new revision's head commits
	// 2. Build line-number mapping
	// 3. Apply mapping to comment's start_line/end_line
	// 4. Read mapped lines and compute Levenshtein ratio
	// 5. Accept if ratio >= 0.5
	return false
}

// ComputeFingerprint computes a SHA-256 fingerprint of the context around a comment.
func ComputeFingerprint(lines []string, startLine, endLine int) string {
	contextStart := startLine - contextRadius
	if contextStart < 0 {
		contextStart = 0
	}
	contextEnd := endLine + contextRadius
	if contextEnd > len(lines) {
		contextEnd = len(lines)
	}

	var normalized []string
	for i := contextStart; i < contextEnd; i++ {
		normalized = append(normalized, strings.TrimSpace(lines[i]))
	}

	joined := strings.Join(normalized, "\n")
	hash := sha256.Sum256([]byte(joined))
	return fmt.Sprintf("%x", hash)
}

// Levenshtein confidence check using the analysis package
func levenshteinConfidence(a, b string) float64 {
	return analysis.LevenshteinRatio(a, b)
}
