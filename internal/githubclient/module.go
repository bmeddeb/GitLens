package githubclient

import "go.uber.org/fx"

var Module = fx.Module("githubclient",
	fx.Provide(NewGitHubClientFactory),
)
