package handlers

import "go.uber.org/fx"

var Module = fx.Module("handlers",
	fx.Provide(
		// Pages (HTML frontend)
		NewPageHandler,
		// Phase 1
		NewHealthHandler,
		NewUserHandler,
		NewJobHandler,
		// Phase 2
		NewRepoHandler,
		NewAnalysisHandler,
		// Phase 3
		NewForkHandler,
		// Phase 4
		NewReviewHandler,
		// Phase 5
		NewNotificationHandler,
		NewPreferenceHandler,
		NewPathOwnerHandler,
		NewAdminHandler,
	),
	fx.Invoke(RegisterRoutes),
)
