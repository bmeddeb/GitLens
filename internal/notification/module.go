package notification

import "go.uber.org/fx"

var Module = fx.Module("notification",
	fx.Provide(
		NewRepository,
		NewService,
		NewPathOwnerService,
		NewPreferenceService,
	),
)
