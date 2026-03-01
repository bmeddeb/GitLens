package handlers

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/user/gitlens-pro/internal/auth"
	apphttp "github.com/user/gitlens-pro/internal/http"
	"github.com/user/gitlens-pro/internal/jobs"
	"github.com/user/gitlens-pro/internal/repository"
)

type ForkHandler struct {
	repoService repository.RepoService
	forkRepo    repository.ForkRepository
	jobService  jobs.Service
}

func NewForkHandler(
	repoService repository.RepoService,
	forkRepo repository.ForkRepository,
	jobService jobs.Service,
) *ForkHandler {
	return &ForkHandler{
		repoService: repoService,
		forkRepo:    forkRepo,
		jobService:  jobService,
	}
}

func (h *ForkHandler) ListForks(w http.ResponseWriter, r *http.Request) {
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

	forks, err := h.forkRepo.ListBySourceRepo(r.Context(), repoID, cursor, limit)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	var nextCursor string
	if len(forks) > limit {
		nextCursor = apphttp.EncodeCursor(forks[limit-1].ID)
		forks = forks[:limit]
	}

	apphttp.SuccessWithMeta(w, forks, &apphttp.Meta{
		NextCursor: nextCursor,
		Limit:      limit,
	})
}

func (h *ForkHandler) AnalyzeFork(w http.ResponseWriter, r *http.Request) {
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

	forkID, err := parseUint64Param(r, "forkID")
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid fork ID"))
		return
	}

	fork, err := h.forkRepo.FindByID(r.Context(), forkID)
	if err != nil || fork == nil {
		apphttp.RespondError(w, apphttp.ErrNotFound)
		return
	}

	fid := forkID
	job, deduped, err := h.jobService.Enqueue(r.Context(), jobs.JobTypeForkAnalysis, repoID, &fid, u.ID)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	if deduped {
		w.Header().Set("X-Job-Deduplicated", "true")
	}
	apphttp.Accepted(w, job, fmt.Sprintf("/api/jobs/%d", job.ID))
}

func (h *ForkHandler) ListForkChanges(w http.ResponseWriter, r *http.Request) {
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

	forkID, err := parseUint64Param(r, "forkID")
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid fork ID"))
		return
	}

	cursor, limit, err := apphttp.ParsePagination(r)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid pagination"))
		return
	}

	changes, err := h.forkRepo.ListForkChanges(r.Context(), forkID, cursor, limit)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	var nextCursor string
	if len(changes) > limit {
		nextCursor = apphttp.EncodeCursor(changes[limit-1].ID)
		changes = changes[:limit]
	}

	apphttp.SuccessWithMeta(w, changes, &apphttp.Meta{
		NextCursor: nextCursor,
		Limit:      limit,
	})
}

func (h *ForkHandler) GetForkReport(w http.ResponseWriter, r *http.Request) {
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

	// Get all forks with complete analysis
	forks, err := h.forkRepo.ListBySourceRepo(r.Context(), repoID, 0, 1000)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	type forkSummary struct {
		Fork    repository.Fork `json:"fork"`
		Changes int             `json:"total_changes"`
	}

	var report []forkSummary
	for _, f := range forks {
		changes, _ := h.forkRepo.ListForkChanges(r.Context(), f.ID, 0, 1000)
		report = append(report, forkSummary{
			Fork:    f,
			Changes: len(changes),
		})
	}

	forkIDParam := chi.URLParam(r, "forkID")
	_ = forkIDParam

	apphttp.Success(w, map[string]interface{}{
		"repo_id":    repoID,
		"fork_count": len(forks),
		"forks":      report,
	})
}
