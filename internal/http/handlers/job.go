package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/user/gitlens-pro/internal/auth"
	apphttp "github.com/user/gitlens-pro/internal/http"
	"github.com/user/gitlens-pro/internal/jobs"
)

type JobHandler struct {
	jobService jobs.Service
}

func NewJobHandler(jobService jobs.Service) *JobHandler {
	return &JobHandler{jobService: jobService}
}

func (h *JobHandler) ListUserJobs(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	cursor, limit, err := apphttp.ParsePagination(r)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid pagination"))
		return
	}

	result, err := h.jobService.ListByUser(r.Context(), u.ID, cursor, limit)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	var nextCursor string
	if len(result) > limit {
		nextCursor = apphttp.EncodeCursor(result[limit-1].ID)
		result = result[:limit]
	}

	apphttp.SuccessWithMeta(w, result, &apphttp.Meta{
		NextCursor: nextCursor,
		Limit:      limit,
	})
}

func (h *JobHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	jobID, err := strconv.ParseUint(chi.URLParam(r, "jobID"), 10, 64)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid job ID"))
		return
	}

	job, err := h.jobService.Get(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, jobs.ErrJobNotFound) {
			apphttp.RespondError(w, apphttp.ErrNotFound)
			return
		}
		apphttp.RespondError(w, err)
		return
	}

	apphttp.Success(w, job)
}

func (h *JobHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	jobID, err := strconv.ParseUint(chi.URLParam(r, "jobID"), 10, 64)
	if err != nil {
		apphttp.RespondError(w, apphttp.NewValidationError("invalid job ID"))
		return
	}

	if err := h.jobService.Cancel(r.Context(), jobID, u.ID); err != nil {
		if errors.Is(err, jobs.ErrJobNotFound) {
			apphttp.RespondError(w, apphttp.ErrNotFound)
			return
		}
		apphttp.RespondError(w, apphttp.NewForbiddenError(err.Error()))
		return
	}

	apphttp.NoContent(w)
}
