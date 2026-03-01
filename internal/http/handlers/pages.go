package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/user/gitlens-pro/internal/analysis"
	analysistemplates "github.com/user/gitlens-pro/internal/analysis/templates"
	"github.com/user/gitlens-pro/internal/auth"
	authtemplates "github.com/user/gitlens-pro/internal/auth/templates"
	"github.com/user/gitlens-pro/internal/gitclient"
	"github.com/user/gitlens-pro/internal/http/templates"
	"github.com/user/gitlens-pro/internal/jobs"
	jobtemplates "github.com/user/gitlens-pro/internal/jobs/templates"
	"github.com/user/gitlens-pro/internal/notification"
	"github.com/user/gitlens-pro/internal/repository"
	repotemplates "github.com/user/gitlens-pro/internal/repository/templates"
)

type PageHandler struct {
	notifService  notification.Service
	repoService   repository.RepoService
	jobService    jobs.Service
	commitRepo    repository.CommitRepository
	contribRepo   repository.ContributorRepository
	repoRepo      repository.RepoRepository
	gitService    gitclient.GitService
	blameAnalyzer analysis.BlameAnalyzer
}

func NewPageHandler(
	notifService notification.Service,
	repoService repository.RepoService,
	jobService jobs.Service,
	commitRepo repository.CommitRepository,
	contribRepo repository.ContributorRepository,
	repoRepo repository.RepoRepository,
	gitService gitclient.GitService,
	blameAnalyzer analysis.BlameAnalyzer,
) *PageHandler {
	return &PageHandler{
		notifService:  notifService,
		repoService:   repoService,
		jobService:    jobService,
		commitRepo:    commitRepo,
		contribRepo:   contribRepo,
		repoRepo:      repoRepo,
		gitService:    gitService,
		blameAnalyzer: blameAnalyzer,
	}
}

// pageData builds common PageData for any page.
func (h *PageHandler) pageData(r *http.Request, title string, path string) templates.PageData {
	u := auth.UserFromContext(r.Context())
	var unread int64
	if u != nil {
		unread, _ = h.notifService.CountUnread(r.Context(), u.ID)
	}
	return templates.PageData{
		Title:       title,
		User:        u,
		CurrentPath: path,
		UnreadCount: int(unread),
	}
}

func html(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}

// --- Dashboard ---

func (h *PageHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	pd := h.pageData(r, "Dashboard", "/")
	html(w)
	if isHTMX(r) {
		templates.DashboardContent(pd).Render(r.Context(), w)
	} else {
		templates.DashboardPage(pd).Render(r.Context(), w)
	}
}

// --- Auth ---

func (h *PageHandler) Login(w http.ResponseWriter, r *http.Request) {
	if u := auth.UserFromContext(r.Context()); u != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	html(w)
	authtemplates.LoginPage().Render(r.Context(), w)
}

// --- Repositories ---

func (h *PageHandler) RepoList(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	pd := h.pageData(r, "Repositories", "/repos")

	repos, _ := h.repoService.ListUserRepos(r.Context(), u.ID, "active", 0, 50)

	var nextCursor string
	if len(repos) > 50 {
		nextCursor = fmt.Sprintf("%d", repos[49].ID)
		repos = repos[:50]
	}

	html(w)
	if isHTMX(r) {
		repotemplates.RepoListContent(repos, nextCursor).Render(r.Context(), w)
	} else {
		repotemplates.RepoListPage(pd, repos, nextCursor).Render(r.Context(), w)
	}
}

func (h *PageHandler) RepoDetail(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	repoID, err := strconv.ParseUint(chi.URLParam(r, "repoID"), 10, 64)
	if err != nil {
		h.Error404(w, r)
		return
	}

	repo, err := h.repoService.GetRepo(r.Context(), u.ID, repoID)
	if err != nil {
		h.Error404(w, r)
		return
	}

	ur, _ := h.repoService.ValidateAccess(r.Context(), u.ID, repoID)
	pd := h.pageData(r, repo.FullName, "/repos")

	html(w)
	if isHTMX(r) {
		repotemplates.RepoDetailContent(pd, repo, ur).Render(r.Context(), w)
	} else {
		repotemplates.RepoDetailPage(pd, repo, ur).Render(r.Context(), w)
	}
}

func (h *PageHandler) RepoAdd(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())

	fullName := r.FormValue("full_name")
	if fullName == "" {
		html(w)
		repotemplates.RepoAddError("Repository name is required.").Render(r.Context(), w)
		return
	}

	repo, jobID, _, err := h.repoService.AddRepo(r.Context(), u.ID, fullName)
	if err != nil {
		html(w)
		repotemplates.RepoAddError(err.Error()).Render(r.Context(), w)
		return
	}

	html(w)
	repotemplates.RepoAddSuccess(repo.FullName, jobID).Render(r.Context(), w)
}

func (h *PageHandler) RepoAddForm(w http.ResponseWriter, r *http.Request) {
	html(w)
	repotemplates.RepoAddForm().Render(r.Context(), w)
}

func (h *PageHandler) RepoCardsFragment(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	cursorStr := r.URL.Query().Get("cursor")
	cursor, _ := strconv.ParseUint(cursorStr, 10, 64)

	repos, _ := h.repoService.ListUserRepos(r.Context(), u.ID, "active", cursor, 50)

	var nextCursor string
	if len(repos) > 50 {
		nextCursor = fmt.Sprintf("%d", repos[49].ID)
		repos = repos[:50]
	}

	html(w)
	repotemplates.RepoCards(repos, nextCursor).Render(r.Context(), w)
}

// --- Jobs ---

func (h *PageHandler) JobList(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	pd := h.pageData(r, "Jobs", "/jobs")

	jobList, _ := h.jobService.ListByUser(r.Context(), u.ID, 0, 50)

	var nextCursor string
	if len(jobList) > 50 {
		nextCursor = fmt.Sprintf("%d", jobList[49].ID)
		jobList = jobList[:50]
	}

	html(w)
	if isHTMX(r) {
		jobtemplates.JobListContent(jobList, nextCursor).Render(r.Context(), w)
	} else {
		jobtemplates.JobListPage(pd, jobList, nextCursor).Render(r.Context(), w)
	}
}

func (h *PageHandler) JobRowFragment(w http.ResponseWriter, r *http.Request) {
	jobID, err := strconv.ParseUint(chi.URLParam(r, "jobID"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	job, err := h.jobService.Get(r.Context(), jobID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	html(w)
	jobtemplates.JobRow(*job).Render(r.Context(), w)
}

func (h *PageHandler) JobRowsFragment(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	cursorStr := r.URL.Query().Get("cursor")
	cursor, _ := strconv.ParseUint(cursorStr, 10, 64)

	jobList, _ := h.jobService.ListByUser(r.Context(), u.ID, cursor, 50)

	var nextCursor string
	if len(jobList) > 50 {
		nextCursor = fmt.Sprintf("%d", jobList[49].ID)
		jobList = jobList[:50]
	}

	html(w)
	jobtemplates.JobRows(jobList, nextCursor).Render(r.Context(), w)
}

// --- Commits ---

func (h *PageHandler) CommitList(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	repoID, err := strconv.ParseUint(chi.URLParam(r, "repoID"), 10, 64)
	if err != nil {
		h.Error404(w, r)
		return
	}

	repo, err := h.repoService.GetRepo(r.Context(), u.ID, repoID)
	if err != nil {
		h.Error404(w, r)
		return
	}

	cursorStr := r.URL.Query().Get("cursor")
	cursor, _ := strconv.ParseUint(cursorStr, 10, 64)

	commits, _ := h.commitRepo.List(r.Context(), repoID, cursor, 50)

	var nextCursor string
	if len(commits) > 50 {
		nextCursor = fmt.Sprintf("%d", commits[49].ID)
		commits = commits[:50]
	}

	pd := h.pageData(r, "Commits — "+repo.FullName, "/repos")
	html(w)
	if isHTMX(r) {
		analysistemplates.CommitListContent(repoID, repo, commits, nextCursor).Render(r.Context(), w)
	} else {
		analysistemplates.CommitListPage(pd, repoID, repo, commits, nextCursor).Render(r.Context(), w)
	}
}

func (h *PageHandler) CommitDetail(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	repoID, err := strconv.ParseUint(chi.URLParam(r, "repoID"), 10, 64)
	if err != nil {
		h.Error404(w, r)
		return
	}

	repo, err := h.repoService.GetRepo(r.Context(), u.ID, repoID)
	if err != nil {
		h.Error404(w, r)
		return
	}

	sha := chi.URLParam(r, "sha")
	commit, err := h.commitRepo.FindBySHA(r.Context(), repoID, sha)
	if err != nil || commit == nil {
		h.Error404(w, r)
		return
	}

	view := r.URL.Query().Get("view")

	// Get diff if repo is cloned
	var diffResult *gitclient.DiffResult
	if repo.LocalPath != nil && len(commit.ParentSHAs) > 0 {
		parentSHA := string(commit.ParentSHAs[0])
		diffResult, _ = h.gitService.Diff(r.Context(), *repo.LocalPath, parentSHA, commit.SHA)
	}

	pd := h.pageData(r, commit.SHA[:7]+" — "+repo.FullName, "/repos")
	html(w)
	if isHTMX(r) {
		// If view toggle request, return just the diff fragment
		if r.URL.Query().Get("view") != "" && r.Header.Get("HX-Target") == "diff-container" {
			analysistemplates.DiffFragment(diffResult, view).Render(r.Context(), w)
			return
		}
		analysistemplates.CommitDetailContent(repoID, repo, commit, diffResult, view).Render(r.Context(), w)
	} else {
		analysistemplates.CommitDetailPage(pd, repoID, repo, commit, diffResult, view).Render(r.Context(), w)
	}
}

func (h *PageHandler) CommitRowsFragment(w http.ResponseWriter, r *http.Request) {
	repoID, err := strconv.ParseUint(chi.URLParam(r, "repoID"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	cursorStr := r.URL.Query().Get("cursor")
	cursor, _ := strconv.ParseUint(cursorStr, 10, 64)

	commits, _ := h.commitRepo.List(r.Context(), repoID, cursor, 50)

	var nextCursor string
	if len(commits) > 50 {
		nextCursor = fmt.Sprintf("%d", commits[49].ID)
		commits = commits[:50]
	}

	html(w)
	analysistemplates.CommitRows(repoID, commits, nextCursor).Render(r.Context(), w)
}

// --- Contributors ---

func (h *PageHandler) ContributorList(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	repoID, err := strconv.ParseUint(chi.URLParam(r, "repoID"), 10, 64)
	if err != nil {
		h.Error404(w, r)
		return
	}

	repo, err := h.repoService.GetRepo(r.Context(), u.ID, repoID)
	if err != nil {
		h.Error404(w, r)
		return
	}

	cursorStr := r.URL.Query().Get("cursor")
	cursor, _ := strconv.ParseUint(cursorStr, 10, 64)

	stats, _ := h.contribRepo.ListStatsByRepo(r.Context(), repoID, cursor, 50)

	var nextCursor string
	if len(stats) > 50 {
		nextCursor = fmt.Sprintf("%d", stats[49].ID)
		stats = stats[:50]
	}

	pd := h.pageData(r, "Contributors — "+repo.FullName, "/repos")
	html(w)
	if isHTMX(r) {
		analysistemplates.ContributorListContent(repoID, repo, stats, nextCursor).Render(r.Context(), w)
	} else {
		analysistemplates.ContributorListPage(pd, repoID, repo, stats, nextCursor).Render(r.Context(), w)
	}
}

func (h *PageHandler) ContributorRowsFragment(w http.ResponseWriter, r *http.Request) {
	repoID, err := strconv.ParseUint(chi.URLParam(r, "repoID"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	cursorStr := r.URL.Query().Get("cursor")
	cursor, _ := strconv.ParseUint(cursorStr, 10, 64)

	stats, _ := h.contribRepo.ListStatsByRepo(r.Context(), repoID, cursor, 50)

	var nextCursor string
	if len(stats) > 50 {
		nextCursor = fmt.Sprintf("%d", stats[49].ID)
		stats = stats[:50]
	}

	html(w)
	analysistemplates.ContributorRows(repoID, stats, nextCursor).Render(r.Context(), w)
}

// --- Blame ---

func (h *PageHandler) BlameView(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	repoID, err := strconv.ParseUint(chi.URLParam(r, "repoID"), 10, 64)
	if err != nil {
		h.Error404(w, r)
		return
	}

	repo, err := h.repoService.GetRepo(r.Context(), u.ID, repoID)
	if err != nil {
		h.Error404(w, r)
		return
	}

	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		h.Error404(w, r)
		return
	}

	ref := r.URL.Query().Get("ref")
	if ref == "" {
		ref = "HEAD"
	}

	var result *gitclient.BlameResult
	if repo.LocalPath != nil {
		result, _ = h.blameAnalyzer.GetBlame(r.Context(), repoID, *repo.LocalPath, filePath, ref)
	}

	pd := h.pageData(r, "Blame — "+filePath, "/repos")
	html(w)
	if isHTMX(r) {
		analysistemplates.BlameViewContent(repoID, repo, filePath, ref, result).Render(r.Context(), w)
	} else {
		analysistemplates.BlameViewPage(pd, repoID, repo, filePath, ref, result).Render(r.Context(), w)
	}
}

// --- Fragments ---

func (h *PageHandler) NotificationBadgeFragment(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var count int64
	if u != nil {
		count, _ = h.notifService.CountUnread(r.Context(), u.ID)
	}
	html(w)
	templates.NotificationBadge(int(count)).Render(r.Context(), w)
}

// --- Errors ---

func (h *PageHandler) Error404(w http.ResponseWriter, r *http.Request) {
	html(w)
	w.WriteHeader(http.StatusNotFound)
	templates.Error404().Render(r.Context(), w)
}

func (h *PageHandler) Error403(w http.ResponseWriter, r *http.Request) {
	html(w)
	w.WriteHeader(http.StatusForbidden)
	templates.Error403().Render(r.Context(), w)
}

func (h *PageHandler) Error500(w http.ResponseWriter, r *http.Request) {
	html(w)
	w.WriteHeader(http.StatusInternalServerError)
	templates.Error500().Render(r.Context(), w)
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
