package mailer

import (
	"context"
	"fmt"

	"github.com/user/gitlens-pro/internal/config"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("mailer",
	fx.Provide(
		provideSender,
		NewNotificationWorker,
		NewDigestScheduler,
	),
	fx.Invoke(registerHooks),
)

func provideSender(cfg *config.AppConfig, logger *zap.Logger) (Sender, error) {
	switch cfg.Email.Provider {
	case "smtp", "":
		logger.Info("using SMTP email sender",
			zap.String("host", cfg.Email.SMTP.Host),
			zap.Int("port", cfg.Email.SMTP.Port),
		)
		return NewSMTPSender(cfg.Email.SMTP), nil
	case "ses":
		logger.Info("using SES email sender", zap.String("region", cfg.Email.SES.Region))
		return NewSESSender(cfg.Email.SES)
	default:
		return nil, fmt.Errorf("unknown email provider: %s", cfg.Email.Provider)
	}
}

func registerHooks(lc fx.Lifecycle, worker *NotificationWorker, digest *DigestScheduler) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := worker.Start(ctx); err != nil {
				return err
			}
			return digest.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			_ = digest.Stop(ctx)
			return worker.Stop(ctx)
		},
	})
}
