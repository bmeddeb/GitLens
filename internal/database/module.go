package database

import (
	"context"

	"github.com/user/gitlens-pro/internal/config"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var Module = fx.Module("database",
	fx.Provide(NewDB),
	fx.Invoke(registerHooks),
)

func registerHooks(lc fx.Lifecycle, db *gorm.DB, cfg *config.AppConfig, logger *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := RunMigrations(cfg, logger); err != nil {
				return err
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("closing database connection")
			return Close(db)
		},
	})
}
