package mailer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/user/gitlens-pro/internal/config"
	"github.com/user/gitlens-pro/internal/notification"
	"go.uber.org/zap"
)

const (
	maxEmailAttempts = 3
	pollInterval     = 10 * time.Second
)

// Exponential backoff: 1m, 5m, 25m
var retryDelays = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	25 * time.Minute,
}

type NotificationWorker struct {
	sender   Sender
	notifRepo notification.Repository
	cfg      *config.AppConfig
	logger   *zap.Logger
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewNotificationWorker(
	sender Sender,
	notifRepo notification.Repository,
	cfg *config.AppConfig,
	logger *zap.Logger,
) *NotificationWorker {
	return &NotificationWorker{
		sender:    sender,
		notifRepo: notifRepo,
		cfg:       cfg,
		logger:    logger,
	}
}

func (w *NotificationWorker) Start(ctx context.Context) error {
	workerCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	w.wg.Add(1)
	go w.loop(workerCtx)

	w.logger.Info("notification worker started")
	return nil
}

func (w *NotificationWorker) Stop(ctx context.Context) error {
	w.logger.Info("stopping notification worker")
	if w.cancel != nil {
		w.cancel()
	}

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.Info("notification worker stopped")
	case <-time.After(30 * time.Second):
		w.logger.Warn("notification worker stop timed out")
	}

	return nil
}

func (w *NotificationWorker) loop(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *NotificationWorker) processBatch(ctx context.Context) {
	notifications, err := w.notifRepo.ListPendingEmails(ctx, 50)
	if err != nil {
		w.logger.Error("failed to fetch pending emails", zap.Error(err))
		return
	}

	for _, n := range notifications {
		if ctx.Err() != nil {
			return
		}
		w.processOne(ctx, &n)
	}
}

func (w *NotificationWorker) processOne(ctx context.Context, n *notification.Notification) {
	msg := &Message{
		From:    w.cfg.Email.From,
		To:      "", // Would resolve user email from user service
		Subject: n.Title,
		HTML:    fmt.Sprintf("<p>%s</p>", n.Title),
		Text:    n.Title,
		Headers: make(map[string]string),
	}

	// Add threading headers
	if n.ReviewID != nil {
		msg.Headers["X-GitLens-Review-ID"] = fmt.Sprintf("%d", *n.ReviewID)
		msgID := fmt.Sprintf("<review-%d@gitlens.example.com>", *n.ReviewID)
		msg.Headers["In-Reply-To"] = msgID
		msg.Headers["References"] = msgID
	}

	err := w.sender.Send(ctx, msg)
	if err != nil {
		n.EmailAttempts++
		if n.EmailAttempts >= maxEmailAttempts {
			n.EmailState = "failed"
			w.logger.Error("email delivery failed permanently",
				zap.Uint64("notification_id", n.ID),
				zap.Error(err),
			)
		}
		// Retry on next poll cycle
		_ = w.notifRepo.Update(ctx, n)
		return
	}

	now := time.Now()
	n.EmailState = "sent"
	n.SentAt = &now
	_ = w.notifRepo.Update(ctx, n)
}
