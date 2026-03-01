package gitclient

import "go.uber.org/fx"

var Module = fx.Module("gitclient",
	fx.Provide(NewGitService),
)
