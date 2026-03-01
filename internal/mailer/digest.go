package mailer

import (
	"context"
	"sync"
	"time"

	"github.com/user/gitlens-pro/internal/config"
	"go.uber.org/zap"
)

type DigestScheduler struct {
	cfg    *config.AppConfig
	logger *zap.Logger
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewDigestScheduler(cfg *config.AppConfig, logger *zap.Logger) *DigestScheduler {
	return &DigestScheduler{cfg: cfg, logger: logger}
}

func (d *DigestScheduler) Start(ctx context.Context) error {
	schedulerCtx, cancel := context.WithCancel(ctx)
	d.cancel = cancel

	d.wg.Add(1)
	go d.loop(schedulerCtx)

	d.logger.Info("digest scheduler started", zap.String("cron", d.cfg.Email.DigestCron))
	return nil
}

func (d *DigestScheduler) Stop(ctx context.Context) error {
	if d.cancel != nil {
		d.cancel()
	}
	d.wg.Wait()
	return nil
}

func (d *DigestScheduler) loop(ctx context.Context) {
	defer d.wg.Done()

	// Simple daily check at configured time
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check if it's digest time (default 8 AM)
			now := time.Now()
			if now.Hour() == 8 && now.Minute() < 5 {
				d.sendDigests(ctx)
			}
		}
	}
}

func (d *DigestScheduler) sendDigests(ctx context.Context) {
	d.logger.Info("running digest aggregation")
	// Would aggregate unread notifications for digest-mode users
	// and send combined emails
}
