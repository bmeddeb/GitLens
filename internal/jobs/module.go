package jobs

import (
	"context"

	"go.uber.org/fx"
)

var Module = fx.Module("jobs",
	fx.Provide(
		NewRepository,
		NewService,
		NewWorkerRegistry,
		NewDispatcher,
		NewCloneWorker,
		NewSyncWorker,
		NewForkAnalysisWorker,
	),
	fx.Invoke(registerWorkers),
	fx.Invoke(registerHooks),
)

func registerWorkers(
	registry *WorkerRegistry,
	cloneWorker *CloneWorker,
	syncWorker *SyncWorker,
	forkWorker *ForkAnalysisWorker,
) {
	registry.Register(JobTypeClone, cloneWorker.Execute)
	registry.Register(JobTypeSync, syncWorker.Execute)
	registry.Register(JobTypeForkAnalysis, forkWorker.Execute)
}

func registerHooks(lc fx.Lifecycle, d *Dispatcher) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return d.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return d.Stop(ctx)
		},
	})
}
