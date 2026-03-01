package database

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/user/gitlens-pro/internal/config"
	"go.uber.org/zap"
)

func RunMigrations(cfg *config.AppConfig, logger *zap.Logger) error {
	dsn := fmt.Sprintf("mysql://%s", cfg.Database.DSN())
	source := fmt.Sprintf("file://%s", cfg.Database.MigrationsPath)

	m, err := migrate.New(source, dsn)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}

	version, dirty, _ := m.Version()
	logger.Info("migrations applied",
		zap.Uint("version", version),
		zap.Bool("dirty", dirty),
	)

	return nil
}
