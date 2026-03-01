package http

import (
	"context"

	"go.uber.org/fx"
)

var Module = fx.Module("http",
	fx.Provide(
		NewRouter,
		NewServer,
	),
	fx.Invoke(registerHooks),
)

func registerHooks(lc fx.Lifecycle, srv *Server) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go srv.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Stop(ctx)
		},
	})
}
