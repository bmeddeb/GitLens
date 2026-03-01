package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/user/gitlens-pro/internal/auth"
	apphttp "github.com/user/gitlens-pro/internal/http"
	"github.com/user/gitlens-pro/internal/repository"
	"github.com/user/gitlens-pro/internal/review"
)

type ReviewHandler struct {
	reviewService      review.Service
	participantService review.ParticipantService
	commentService     review.CommentService
	repoService        repository.RepoService
}

func NewReviewHandler(
	reviewService review.Service,
	participantService review.ParticipantService,
	commentService review.CommentService,
	repoService repository.RepoService,
) *ReviewHandler {
	return &ReviewHandler{
		reviewService:      reviewService,
		participantService: participantService,
		commentService:     commentService,
		repoService:        repoService,
	}
}

// Review CRUD

func (h *ReviewHandler) ListReviews(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	repoID, err := parseRepoID(r)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	if _, err := h.repoService.ValidateAccess(r.Context(), u.ID, repoID); err != nil {
		apphttp.RespondError(w, apphttp.NewForbiddenError(err.Error()))
		return
	}

	cursor, limit, err := apphttp.ParsePagination(r)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid pagination"))
		return
	}

	reviews, err := h.reviewService.ListReviews(r.Context(), repoID, cursor, limit)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	var nextCursor string
	if len(reviews) > limit {
		nextCursor = apphttp.EncodeCursor(reviews[limit-1].ID)
		reviews = reviews[:limit]
	}

	apphttp.SuccessWithMeta(w, reviews, &apphttp.Meta{
		NextCursor: nextCursor,
		Limit:      limit,
	})
}

type createReviewRequest struct {
	Kind          string `json:"kind"`
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	OriginType    string `json:"origin_type"`
	OriginPayload string `json:"origin_payload"`
	Draft         bool   `json:"draft"`
	BaseCommitSHA string `json:"base_commit_sha"`
	HeadCommitSHA string `json:"head_commit_sha"`
}

func (h *ReviewHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	repoID, err := parseRepoID(r)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	if _, err := h.repoService.ValidateAccess(r.Context(), u.ID, repoID); err != nil {
		apphttp.RespondError(w, apphttp.NewForbiddenError(err.Error()))
		return
	}

	var req createReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid request body"))
		return
	}

	rev, err := h.reviewService.CreateReview(r.Context(), review.CreateReviewRequest{
		RepoID:        repoID,
		Kind:          req.Kind,
		Title:         req.Title,
		Summary:       req.Summary,
		OriginType:    req.OriginType,
		OriginPayload: req.OriginPayload,
		AuthorID:      u.ID,
		Draft:         req.Draft,
		BaseCommitSHA: req.BaseCommitSHA,
		HeadCommitSHA: req.HeadCommitSHA,
	})
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	apphttp.Created(w, rev)
}

func (h *ReviewHandler) GetReview(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	reviewID, err := strconv.ParseUint(chi.URLParam(r, "reviewID"), 10, 64)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid review ID"))
		return
	}

	rev, err := h.reviewService.GetReview(r.Context(), reviewID)
	if err != nil {
		if errors.Is(err, review.ErrReviewNotFound) {
			apphttp.RespondError(w, apphttp.ErrNotFound)
			return
		}
		apphttp.RespondError(w, err)
		return
	}

	if _, err := h.repoService.ValidateAccess(r.Context(), u.ID, rev.RepoID); err != nil {
		apphttp.RespondError(w, apphttp.NewForbiddenError(err.Error()))
		return
	}

	apphttp.Success(w, rev)
}

// Revisions

func (h *ReviewHandler) AddRevision(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	reviewID, err := strconv.ParseUint(chi.URLParam(r, "reviewID"), 10, 64)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid review ID"))
		return
	}

	var req review.AddRevisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid request body"))
		return
	}

	rev, err := h.reviewService.AddRevision(r.Context(), reviewID, u.ID, req)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewForbiddenError(err.Error()))
		return
	}

	apphttp.Created(w, rev)
}

// Diff

func (h *ReviewHandler) GetDiff(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	reviewID, err := strconv.ParseUint(chi.URLParam(r, "reviewID"), 10, 64)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid review ID"))
		return
	}

	rev, err := h.reviewService.GetReview(r.Context(), reviewID)
	if err != nil {
		apphttp.RespondError(w, apphttp.ErrNotFound)
		return
	}

	if _, err := h.repoService.ValidateAccess(r.Context(), u.ID, rev.RepoID); err != nil {
		apphttp.RespondError(w, apphttp.NewForbiddenError(err.Error()))
		return
	}

	apphttp.Success(w, map[string]interface{}{
		"review_id": reviewID,
		"revision":  rev.LatestRevision,
		"view":      r.URL.Query().Get("view"),
	})
}

// Participants

func (h *ReviewHandler) AddParticipant(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	reviewID, err := strconv.ParseUint(chi.URLParam(r, "reviewID"), 10, 64)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid review ID"))
		return
	}

	var req struct {
		UserID uint64 `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid request body"))
		return
	}

	// Self-subscribe
	if req.UserID == u.ID && req.Role == "subscriber" {
		p, err := h.participantService.SelfSubscribe(r.Context(), reviewID, u.ID)
		if err != nil {
			apphttp.RespondError(w, err)
			return
		}
		apphttp.Created(w, p)
		return
	}

	p, err := h.participantService.AddParticipant(r.Context(), reviewID, req.UserID, u.ID, req.Role, "manual")
	if err != nil {
		apphttp.RespondError(w, apphttp.NewForbiddenError(err.Error()))
		return
	}

	apphttp.Created(w, p)
}

func (h *ReviewHandler) RemoveParticipant(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	reviewID, err := strconv.ParseUint(chi.URLParam(r, "reviewID"), 10, 64)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid review ID"))
		return
	}

	pid, err := strconv.ParseUint(chi.URLParam(r, "pid"), 10, 64)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid participant ID"))
		return
	}

	if err := h.participantService.RemoveParticipant(r.Context(), reviewID, pid, u.ID); err != nil {
		apphttp.RespondError(w, apphttp.NewForbiddenError(err.Error()))
		return
	}

	apphttp.NoContent(w)
}

// Comments

func (h *ReviewHandler) CreateDraft(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	reviewID, err := strconv.ParseUint(chi.URLParam(r, "reviewID"), 10, 64)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid review ID"))
		return
	}

	var req review.CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid request body"))
		return
	}

	comment, err := h.commentService.CreateDraft(r.Context(), reviewID, u.ID, req)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	apphttp.Created(w, comment)
}

func (h *ReviewHandler) ListComments(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	reviewID, err := strconv.ParseUint(chi.URLParam(r, "reviewID"), 10, 64)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid review ID"))
		return
	}

	cursor, limit, err := apphttp.ParsePagination(r)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid pagination"))
		return
	}

	visibility := r.URL.Query().Get("visibility")
	comments, err := h.commentService.ListComments(r.Context(), reviewID, visibility, u.ID, cursor, limit)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	apphttp.Success(w, comments)
}

type publishRequest struct {
	Decision string `json:"decision"`
}

func (h *ReviewHandler) PublishDrafts(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	reviewID, err := strconv.ParseUint(chi.URLParam(r, "reviewID"), 10, 64)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid review ID"))
		return
	}

	var req publishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid request body"))
		return
	}

	comments, err := h.commentService.PublishDrafts(r.Context(), reviewID, u.ID, req.Decision)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	// Apply decision if present
	if req.Decision != "" {
		if err := h.participantService.SubmitDecision(r.Context(), reviewID, u.ID, req.Decision); err != nil {
			apphttp.RespondError(w, err)
			return
		}
	}

	apphttp.Success(w, comments)
}

func (h *ReviewHandler) ResolveComment(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	cid, err := strconv.ParseUint(chi.URLParam(r, "cid"), 10, 64)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid comment ID"))
		return
	}

	if err := h.commentService.ResolveComment(r.Context(), cid, u.ID); err != nil {
		apphttp.RespondError(w, apphttp.NewForbiddenError(err.Error()))
		return
	}

	apphttp.NoContent(w)
}

func (h *ReviewHandler) ReopenComment(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	cid, err := strconv.ParseUint(chi.URLParam(r, "cid"), 10, 64)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid comment ID"))
		return
	}

	if err := h.commentService.ReopenComment(r.Context(), cid, u.ID); err != nil {
		apphttp.RespondError(w, apphttp.NewForbiddenError(err.Error()))
		return
	}

	apphttp.NoContent(w)
}

func (h *ReviewHandler) SubmitDecision(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	reviewID, err := strconv.ParseUint(chi.URLParam(r, "reviewID"), 10, 64)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid review ID"))
		return
	}

	var req struct {
		Decision string `json:"decision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid request body"))
		return
	}

	if err := h.participantService.SubmitDecision(r.Context(), reviewID, u.ID, req.Decision); err != nil {
		apphttp.RespondError(w, apphttp.NewForbiddenError(err.Error()))
		return
	}

	apphttp.NoContent(w)
}

// Suppress unused import
var _ = fmt.Sprintf
