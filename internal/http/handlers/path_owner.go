package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/user/gitlens-pro/internal/auth"
	apphttp "github.com/user/gitlens-pro/internal/http"
	"github.com/user/gitlens-pro/internal/notification"
	"github.com/user/gitlens-pro/internal/repository"
)

type PathOwnerHandler struct {
	pathOwnerService notification.PathOwnerService
	repoService      repository.RepoService
}

func NewPathOwnerHandler(
	pathOwnerService notification.PathOwnerService,
	repoService repository.RepoService,
) *PathOwnerHandler {
	return &PathOwnerHandler{
		pathOwnerService: pathOwnerService,
		repoService:      repoService,
	}
}

func (h *PathOwnerHandler) ListRules(w http.ResponseWriter, r *http.Request) {
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

	rules, err := h.pathOwnerService.ListRules(r.Context(), repoID)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	apphttp.Success(w, rules)
}

type createRuleRequest struct {
	PathGlob     string  `json:"path_glob"`
	FileExt      *string `json:"file_ext,omitempty"`
	TargetUserID uint64  `json:"target_user_id"`
	ActionRole   string  `json:"action_role"`
}

func (h *PathOwnerHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
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

	var req createRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid request body"))
		return
	}

	// Validate target user has active repo access
	if _, err := h.repoService.ValidateAccess(r.Context(), req.TargetUserID, repoID); err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("target user must have active repository access"))
		return
	}

	rule := &notification.PathOwnerRule{
		RepoID:       repoID,
		PathGlob:     req.PathGlob,
		FileExt:      req.FileExt,
		TargetUserID: req.TargetUserID,
		ActionRole:   req.ActionRole,
		IsActive:     true,
		CreatedBy:    u.ID,
	}

	if err := h.pathOwnerService.CreateRule(r.Context(), rule); err != nil {
		apphttp.RespondError(w, err)
		return
	}

	apphttp.Created(w, rule)
}

func (h *PathOwnerHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
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

	ruleID, err := strconv.ParseUint(chi.URLParam(r, "ruleID"), 10, 64)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid rule ID"))
		return
	}

	if err := h.pathOwnerService.DeleteRule(r.Context(), ruleID); err != nil {
		apphttp.RespondError(w, err)
		return
	}

	apphttp.NoContent(w)
}
