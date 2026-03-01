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
	"github.com/user/gitlens-pro/internal/jobs"
	"github.com/user/gitlens-pro/internal/repository"
)

type RepoHandler struct {
	repoService repository.RepoService
	jobService  jobs.Service
}

func NewRepoHandler(repoService repository.RepoService, jobService jobs.Service) *RepoHandler {
	return &RepoHandler{repoService: repoService, jobService: jobService}
}

func (h *RepoHandler) ListRepos(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	state := r.URL.Query().Get("state")

	cursor, limit, err := apphttp.ParsePagination(r)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid pagination"))
		return
	}

	repos, err := h.repoService.ListUserRepos(r.Context(), u.ID, state, cursor, limit)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	var nextCursor string
	if len(repos) > limit {
		nextCursor = apphttp.EncodeCursor(repos[limit-1].ID)
		repos = repos[:limit]
	}

	apphttp.SuccessWithMeta(w, repos, &apphttp.Meta{
		NextCursor: nextCursor,
		Limit:      limit,
	})
}

type addRepoRequest struct {
	FullName string `json:"full_name"`
}

func (h *RepoHandler) AddRepo(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	var req addRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid request body"))
		return
	}

	repo, jobID, deduped, err := h.repoService.AddRepo(r.Context(), u.ID, req.FullName)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	if jobID > 0 {
		if deduped {
			w.Header().Set("X-Job-Deduplicated", "true")
		}
		job, _ := h.jobService.Get(r.Context(), jobID)
		apphttp.Accepted(w, map[string]interface{}{
			"repository": repo,
			"job":        job,
		}, fmt.Sprintf("/api/jobs/%d", jobID))
		return
	}

	apphttp.Created(w, repo)
}

func (h *RepoHandler) GetRepo(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	repoID, err := parseRepoID(r)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	repo, err := h.repoService.GetRepo(r.Context(), u.ID, repoID)
	if err != nil {
		if errors.Is(err, repository.ErrRepoNotFound) {
			apphttp.RespondError(w, apphttp.ErrNotFound)
			return
		}
		apphttp.RespondError(w, apphttp.NewForbiddenError(err.Error()))
		return
	}

	apphttp.Success(w, repo)
}

func (h *RepoHandler) SyncRepo(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	repoID, err := parseRepoID(r)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	jobID, deduped, err := h.repoService.SyncRepo(r.Context(), u.ID, repoID)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewForbiddenError(err.Error()))
		return
	}

	if deduped {
		w.Header().Set("X-Job-Deduplicated", "true")
	}
	job, _ := h.jobService.Get(r.Context(), jobID)
	apphttp.Accepted(w, job, fmt.Sprintf("/api/jobs/%d", jobID))
}

func (h *RepoHandler) RemoveRepo(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	repoID, err := parseRepoID(r)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	if err := h.repoService.RemoveRepo(r.Context(), u.ID, repoID); err != nil {
		apphttp.RespondError(w, err)
		return
	}

	apphttp.NoContent(w)
}

func (h *RepoHandler) RecheckAccess(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	repoID, err := parseRepoID(r)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	state, err := h.repoService.RecheckAccess(r.Context(), u.ID, repoID)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	apphttp.Success(w, map[string]string{"access_state": state})
}

func parseRepoID(r *http.Request) (uint64, error) {
	id, err := strconv.ParseUint(chi.URLParam(r, "repoID"), 10, 64)
	if err != nil {
		return 0, apphttp.NewValidationError("invalid repo ID")
	}
	return id, nil
}
