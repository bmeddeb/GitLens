package jobs

import (
	"context"
	"sync"
	"time"

	"github.com/user/gitlens-pro/internal/config"
	"go.uber.org/zap"
)

type WorkerFunc func(ctx context.Context, job *Job) error

type WorkerRegistry struct {
	mu       sync.RWMutex
	handlers map[JobType]WorkerFunc
}

func NewWorkerRegistry() *WorkerRegistry {
	return &WorkerRegistry{
		handlers: make(map[JobType]WorkerFunc),
	}
}

func (r *WorkerRegistry) Register(jobType JobType, handler WorkerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[jobType] = handler
}

func (r *WorkerRegistry) Get(jobType JobType) (WorkerFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[jobType]
	return h, ok
}

type Dispatcher struct {
	repo     Repository
	service  Service
	registry *WorkerRegistry
	cfg      *config.AppConfig
	logger   *zap.Logger
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewDispatcher(
	repo Repository,
	svc Service,
	registry *WorkerRegistry,
	cfg *config.AppConfig,
	logger *zap.Logger,
) *Dispatcher {
	return &Dispatcher{
		repo:     repo,
		service:  svc,
		registry: registry,
		cfg:      cfg,
		logger:   logger,
	}
}

func (d *Dispatcher) Start(ctx context.Context) error {
	// Reset stale running jobs
	count, err := d.repo.ResetStaleRunning(ctx, d.cfg.Jobs.StaleRunningTimeout)
	if err != nil {
		return err
	}
	if count > 0 {
		d.logger.Info("reset stale running jobs", zap.Int64("count", count))
	}

	workerCtx, cancel := context.WithCancel(ctx)
	d.cancel = cancel

	for i := 0; i < d.cfg.Jobs.WorkerCount; i++ {
		d.wg.Add(1)
		go d.workerLoop(workerCtx, i)
	}

	d.logger.Info("job dispatcher started", zap.Int("workers", d.cfg.Jobs.WorkerCount))
	return nil
}

func (d *Dispatcher) Stop(ctx context.Context) error {
	d.logger.Info("stopping job dispatcher")
	if d.cancel != nil {
		d.cancel()
	}

	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		d.logger.Info("job dispatcher stopped gracefully")
	case <-time.After(60 * time.Second):
		d.logger.Warn("job dispatcher stop timed out after 60s")
	}

	return nil
}

func (d *Dispatcher) workerLoop(ctx context.Context, workerID int) {
	defer d.wg.Done()

	ticker := time.NewTicker(d.cfg.Jobs.PollInterval)
	defer ticker.Stop()

	jobTypes := []JobType{JobTypeClone, JobTypeSync, JobTypeForkAnalysis}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, jt := range jobTypes {
				if ctx.Err() != nil {
					return
				}
				d.processNextJob(ctx, jt, workerID)
			}
		}
	}
}

func (d *Dispatcher) processNextJob(ctx context.Context, jobType JobType, workerID int) {
	job, err := d.repo.DequeueNext(ctx, jobType)
	if err != nil {
		d.logger.Error("failed to dequeue job",
			zap.String("type", string(jobType)),
			zap.Error(err),
		)
		return
	}
	if job == nil {
		return
	}

	handler, ok := d.registry.Get(jobType)
	if !ok {
		d.logger.Warn("no handler registered for job type", zap.String("type", string(jobType)))
		errMsg := "no handler registered"
		_ = d.service.Fail(ctx, job.ID, errMsg)
		return
	}

	d.logger.Info("processing job",
		zap.Int("worker", workerID),
		zap.Uint64("job_id", job.ID),
		zap.String("type", string(jobType)),
	)

	if err := handler(ctx, job); err != nil {
		d.logger.Error("job failed",
			zap.Uint64("job_id", job.ID),
			zap.Error(err),
		)
		_ = d.service.Fail(ctx, job.ID, err.Error())
		return
	}

	_ = d.service.Complete(ctx, job.ID, nil)
}
