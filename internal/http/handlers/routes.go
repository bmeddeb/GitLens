package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/user/gitlens-pro/internal/auth"
	"go.uber.org/fx"
)

type RouteParams struct {
	fx.In

	Router            *chi.Mux
	AuthHandler       *auth.OAuthHandler
	SessionMiddleware *auth.SessionMiddleware
	Pages             *PageHandler
	Health            *HealthHandler
	User              *UserHandler
	Job               *JobHandler
	Repo              *RepoHandler
	Analysis          *AnalysisHandler
	Fork              *ForkHandler
	Review            *ReviewHandler
	Notification      *NotificationHandler
	Preference        *PreferenceHandler
	PathOwner         *PathOwnerHandler
	Admin             *AdminHandler
}

func RegisterRoutes(p RouteParams) {
	r := p.Router

	// Static files
	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// Public routes
	r.Get("/healthz", p.Health.Healthz)
	r.Get("/readyz", p.Health.Readyz)

	// Auth routes (no session required)
	r.Get("/auth/github/login", p.AuthHandler.Login)
	r.Get("/auth/github/callback", p.AuthHandler.Callback)

	// Public page routes
	r.Get("/login", p.Pages.Login)

	// Protected page routes (redirect to /login on auth failure)
	r.Group(func(r chi.Router) {
		r.Use(p.SessionMiddleware.RequireAuthPage)

		r.Get("/", p.Pages.Dashboard)

		// Repos
		r.Get("/repos", p.Pages.RepoList)
		r.Get("/repos/{repoID}", p.Pages.RepoDetail)
		r.Post("/repos", p.Pages.RepoAdd)

		// Jobs
		r.Get("/jobs", p.Pages.JobList)

		// Commits & Analysis
		r.Get("/repos/{repoID}/commits", p.Pages.CommitList)
		r.Get("/repos/{repoID}/commits/{sha}", p.Pages.CommitDetail)
		r.Get("/repos/{repoID}/contributors", p.Pages.ContributorList)
		r.Get("/repos/{repoID}/blame", p.Pages.BlameView)

		// Forks
		r.Get("/repos/{repoID}/forks", p.Pages.ForkList)
		r.Get("/repos/{repoID}/forks/{forkID}/changes", p.Pages.ForkChanges)
		r.Get("/repos/{repoID}/fork-report", p.Pages.ForkReport)

		// HTMX fragments
		r.Get("/fragments/notification-count", p.Pages.NotificationBadgeFragment)
		r.Get("/fragments/repo-add-form", p.Pages.RepoAddForm)
		r.Get("/fragments/repo-cards", p.Pages.RepoCardsFragment)
		r.Get("/fragments/job-row/{jobID}", p.Pages.JobRowFragment)
		r.Get("/fragments/job-rows", p.Pages.JobRowsFragment)
		r.Get("/fragments/commits/{repoID}", p.Pages.CommitRowsFragment)
		r.Get("/fragments/contributors/{repoID}", p.Pages.ContributorRowsFragment)
		r.Get("/fragments/forks/{repoID}", p.Pages.ForkRowsFragment)
		r.Get("/fragments/fork-row/{repoID}/{forkID}", p.Pages.ForkRowFragment)
		r.Get("/fragments/fork-changes/{repoID}/{forkID}", p.Pages.ForkChangeRowsFragment)
	})

	// Protected API routes
	r.Group(func(r chi.Router) {
		r.Use(p.SessionMiddleware.RequireAuth)

		r.Post("/auth/logout", p.AuthHandler.Logout)
		r.Get("/auth/me", p.User.Me)

		// Jobs
		r.Get("/api/jobs", p.Job.ListUserJobs)
		r.Get("/api/jobs/{jobID}", p.Job.GetJob)
		r.Post("/api/jobs/{jobID}/cancel", p.Job.CancelJob)

		// Repos
		r.Get("/api/repos", p.Repo.ListRepos)
		r.Post("/api/repos", p.Repo.AddRepo)
		r.Get("/api/repos/{repoID}", p.Repo.GetRepo)
		r.Post("/api/repos/{repoID}/sync", p.Repo.SyncRepo)
		r.Delete("/api/repos/{repoID}", p.Repo.RemoveRepo)
		r.Post("/api/repos/{repoID}/recheck-access", p.Repo.RecheckAccess)

		// Analysis
		r.Get("/api/repos/{repoID}/commits", p.Analysis.ListCommits)
		r.Get("/api/repos/{repoID}/commits/{sha}", p.Analysis.GetCommit)
		r.Get("/api/repos/{repoID}/commits/{sha}/diff", p.Analysis.GetCommitDiff)
		r.Get("/api/repos/{repoID}/contributors", p.Analysis.ListContributors)
		r.Get("/api/repos/{repoID}/blame", p.Analysis.GetBlame)
		r.Get("/api/repos/{repoID}/jobs", p.Analysis.ListRepoJobs)

		// Forks
		r.Get("/api/repos/{repoID}/forks", p.Fork.ListForks)
		r.Post("/api/repos/{repoID}/forks/{forkID}/analyze", p.Fork.AnalyzeFork)
		r.Get("/api/repos/{repoID}/forks/{forkID}/changes", p.Fork.ListForkChanges)
		r.Get("/api/repos/{repoID}/fork-report", p.Fork.GetForkReport)

		// Reviews
		r.Get("/api/repos/{repoID}/reviews", p.Review.ListReviews)
		r.Post("/api/repos/{repoID}/reviews", p.Review.CreateReview)
		r.Get("/api/reviews/{reviewID}", p.Review.GetReview)
		r.Post("/api/reviews/{reviewID}/revisions", p.Review.AddRevision)
		r.Get("/api/reviews/{reviewID}/diff", p.Review.GetDiff)
		r.Post("/api/reviews/{reviewID}/participants", p.Review.AddParticipant)
		r.Delete("/api/reviews/{reviewID}/participants/{pid}", p.Review.RemoveParticipant)
		r.Post("/api/reviews/{reviewID}/comments/drafts", p.Review.CreateDraft)
		r.Get("/api/reviews/{reviewID}/comments", p.Review.ListComments)
		r.Post("/api/reviews/{reviewID}/comments/publish", p.Review.PublishDrafts)
		r.Post("/api/reviews/{reviewID}/comments/{cid}/resolve", p.Review.ResolveComment)
		r.Post("/api/reviews/{reviewID}/comments/{cid}/reopen", p.Review.ReopenComment)
		r.Post("/api/reviews/{reviewID}/decision", p.Review.SubmitDecision)

		// Notifications
		r.Get("/api/notifications", p.Notification.ListNotifications)
		r.Post("/api/notifications/{id}/read", p.Notification.MarkRead)
		r.Post("/api/notifications/read-all", p.Notification.MarkAllRead)

		// Preferences
		r.Get("/api/preferences/notifications", p.Preference.GetPreferences)
		r.Put("/api/preferences/notifications", p.Preference.UpsertPreferences)

		// Path owners
		r.Get("/api/repos/{repoID}/path-owners", p.PathOwner.ListRules)
		r.Post("/api/repos/{repoID}/path-owners", p.PathOwner.CreateRule)
		r.Delete("/api/repos/{repoID}/path-owners/{ruleID}", p.PathOwner.DeleteRule)

		// Admin
		r.Get("/api/admin/diagnostics", p.Admin.Diagnostics)
	})
}
