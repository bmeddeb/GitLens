package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/user/gitlens-pro/internal/analysis"
	"github.com/user/gitlens-pro/internal/auth"
	apphttp "github.com/user/gitlens-pro/internal/http"
	"github.com/user/gitlens-pro/internal/repository"
)

type AnalysisHandler struct {
	repoService  repository.RepoService
	commitRepo   repository.CommitRepository
	contribRepo  repository.ContributorRepository
	blameAnalyzer analysis.BlameAnalyzer
	repoRepo     repository.RepoRepository
}

func NewAnalysisHandler(
	repoService repository.RepoService,
	commitRepo repository.CommitRepository,
	contribRepo repository.ContributorRepository,
	blameAnalyzer analysis.BlameAnalyzer,
	repoRepo repository.RepoRepository,
) *AnalysisHandler {
	return &AnalysisHandler{
		repoService:   repoService,
		commitRepo:    commitRepo,
		contribRepo:   contribRepo,
		blameAnalyzer: blameAnalyzer,
		repoRepo:     repoRepo,
	}
}

func (h *AnalysisHandler) ListCommits(w http.ResponseWriter, r *http.Request) {
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

	opts := repository.CommitFilterOptions{
		Author:  r.URL.Query().Get("author"),
		Path:    r.URL.Query().Get("path"),
		Keyword: r.URL.Query().Get("keyword"),
		Cursor:  cursor,
		Limit:   limit,
	}

	commits, err := h.commitRepo.ListFiltered(r.Context(), repoID, opts)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	var nextCursor string
	if len(commits) > limit {
		nextCursor = apphttp.EncodeCursor(commits[limit-1].ID)
		commits = commits[:limit]
	}

	apphttp.SuccessWithMeta(w, commits, &apphttp.Meta{
		NextCursor: nextCursor,
		Limit:      limit,
	})
}

func (h *AnalysisHandler) GetCommit(w http.ResponseWriter, r *http.Request) {
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

	sha := chi.URLParam(r, "sha")
	commit, err := h.commitRepo.FindBySHA(r.Context(), repoID, sha)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}
	if commit == nil {
		apphttp.RespondError(w, apphttp.ErrNotFound)
		return
	}

	apphttp.Success(w, commit)
}

func (h *AnalysisHandler) GetCommitDiff(w http.ResponseWriter, r *http.Request) {
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

	// For now, return the commit info with stats
	sha := chi.URLParam(r, "sha")
	commit, err := h.commitRepo.FindBySHA(r.Context(), repoID, sha)
	if err != nil || commit == nil {
		apphttp.RespondError(w, apphttp.ErrNotFound)
		return
	}

	apphttp.Success(w, map[string]interface{}{
		"commit": commit,
		"view":   r.URL.Query().Get("view"),
	})
}

func (h *AnalysisHandler) ListContributors(w http.ResponseWriter, r *http.Request) {
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

	stats, err := h.contribRepo.ListStatsByRepo(r.Context(), repoID, cursor, limit)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	var nextCursor string
	if len(stats) > limit {
		nextCursor = apphttp.EncodeCursor(stats[limit-1].ID)
		stats = stats[:limit]
	}

	apphttp.SuccessWithMeta(w, stats, &apphttp.Meta{
		NextCursor: nextCursor,
		Limit:      limit,
	})
}

func (h *AnalysisHandler) GetBlame(w http.ResponseWriter, r *http.Request) {
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

	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		apphttp.RespondError(w, apphttp.NewValidationError("path parameter required"))
		return
	}

	ref := r.URL.Query().Get("ref")
	if ref == "" {
		ref = "HEAD"
	}

	repo, err := h.repoRepo.FindByID(r.Context(), repoID)
	if err != nil {
		if errors.Is(err, repository.ErrRepoNotFound) {
			apphttp.RespondError(w, apphttp.ErrNotFound)
			return
		}
		apphttp.RespondError(w, err)
		return
	}

	if repo.LocalPath == nil {
		apphttp.RespondError(w, apphttp.NewAppError("REPO_NOT_READY", "Repository is not yet cloned", http.StatusConflict))
		return
	}

	result, err := h.blameAnalyzer.GetBlame(r.Context(), repoID, *repo.LocalPath, filePath, ref)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	apphttp.Success(w, result)
}

func (h *AnalysisHandler) ListRepoJobs(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	repoID, err := parseRepoID(r)
	if err != nil {
		apphttp.RespondError(w, err)
		return
	}

	// Verify user has repo in workspace (any state)
	if _, err := h.repoService.ValidateAccess(r.Context(), u.ID, repoID); err != nil {
		apphttp.RespondError(w, apphttp.NewForbiddenError(err.Error()))
		return
	}

	// Delegate to job handler — but since we don't have the job service here,
	// return a placeholder
	apphttp.Success(w, []interface{}{})
}

// Helper
func parseUint64Param(r *http.Request, param string) (uint64, error) {
	return strconv.ParseUint(chi.URLParam(r, param), 10, 64)
}
