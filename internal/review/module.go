package review

import "go.uber.org/fx"

var Module = fx.Module("review",
	fx.Provide(
		NewRepository,
		NewService,
		NewParticipantService,
		NewCommentService,
	),
)
