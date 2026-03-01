package main

import (
	"context"

	"github.com/user/gitlens-pro/internal/analysis"
	"github.com/user/gitlens-pro/internal/auth"
	"github.com/user/gitlens-pro/internal/config"
	"github.com/user/gitlens-pro/internal/database"
	"github.com/user/gitlens-pro/internal/gitclient"
	"github.com/user/gitlens-pro/internal/githubclient"
	apphttp "github.com/user/gitlens-pro/internal/http"
	"github.com/user/gitlens-pro/internal/http/handlers"
	"github.com/user/gitlens-pro/internal/jobs"
	"github.com/user/gitlens-pro/internal/logger"
	"github.com/user/gitlens-pro/internal/mailer"
	"github.com/user/gitlens-pro/internal/notification"
	"github.com/user/gitlens-pro/internal/repository"
	"github.com/user/gitlens-pro/internal/review"
	"github.com/user/gitlens-pro/internal/user"
	"go.uber.org/fx"
)

// jobEnqueuerAdapter bridges jobs.Service to repository.JobEnqueuer,
// breaking the import cycle between the two packages.
type jobEnqueuerAdapter struct {
	svc jobs.Service
}

func (a *jobEnqueuerAdapter) Enqueue(ctx context.Context, jobType string, repoID uint64, forkID *uint64, requesterID uint64) (uint64, bool, error) {
	job, deduped, err := a.svc.Enqueue(ctx, jobs.JobType(jobType), repoID, forkID, requesterID)
	if err != nil {
		return 0, false, err
	}
	return job.ID, deduped, nil
}

func provideJobEnqueuer(svc jobs.Service) repository.JobEnqueuer {
	return &jobEnqueuerAdapter{svc: svc}
}

func main() {
	fx.New(
		// Phase 1: Foundation
		config.Module,
		logger.Module,
		database.Module,
		user.Module,
		auth.Module,

		// Phase 2: Git Core
		githubclient.Module,
		gitclient.Module,
		repository.Module,
		analysis.Module,

		// Phase 4: Review Core
		review.Module,

		// Phase 5: Notifications & Mailer
		notification.Module,
		mailer.Module,

		// Jobs (workers depend on analysis, repository, gitclient)
		jobs.Module,

		// Bridge: repository.JobEnqueuer ← jobs.Service
		fx.Provide(provideJobEnqueuer),

		// HTTP
		handlers.Module,
		apphttp.Module,
	).Run()
}
