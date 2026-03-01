package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
)

type AppConfig struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	GitHub     GitHubConfig     `mapstructure:"github"`
	Encryption EncryptionConfig `mapstructure:"encryption"`
	Storage    StorageConfig    `mapstructure:"storage"`
	Jobs       JobsConfig       `mapstructure:"jobs"`
	Access     AccessConfig     `mapstructure:"access"`
	Email      EmailConfig      `mapstructure:"email"`
	Log        LogConfig        `mapstructure:"log"`
}

type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	BaseURL         string        `mapstructure:"base_url"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Name            string        `mapstructure:"name"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	MigrationsPath  string        `mapstructure:"migrations_path"`
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=UTC&multiStatements=true",
		d.User, d.Password, d.Host, d.Port, d.Name)
}

type GitHubConfig struct {
	ClientID     string   `mapstructure:"client_id"`
	ClientSecret string   `mapstructure:"client_secret"`
	Scopes       []string `mapstructure:"scopes"`
}

type EncryptionConfig struct {
	Key string `mapstructure:"key"`
}

type StorageConfig struct {
	ReposPath string `mapstructure:"repos_path"`
	ForksPath string `mapstructure:"forks_path"`
}

type JobsConfig struct {
	WorkerCount         int           `mapstructure:"worker_count"`
	PollInterval        time.Duration `mapstructure:"poll_interval"`
	StaleRunningTimeout time.Duration `mapstructure:"stale_running_timeout"`
}

type AccessConfig struct {
	RateLimitPerMin        int           `mapstructure:"rate_limit_per_min"`
	SessionDuration        time.Duration `mapstructure:"session_duration"`
	RevalidationStaleAfter time.Duration `mapstructure:"revalidation_stale_after"`
}

type EmailConfig struct {
	Provider  string     `mapstructure:"provider"`
	From      string     `mapstructure:"from"`
	SMTP      SMTPConfig `mapstructure:"smtp"`
	SES       SESConfig  `mapstructure:"ses"`
	DigestCron string    `mapstructure:"digest_cron"`
}

type SMTPConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	StartTLS bool   `mapstructure:"starttls"`
}

type SESConfig struct {
	Region string `mapstructure:"region"`
}

type LogConfig struct {
	Level    string `mapstructure:"level"`
	Encoding string `mapstructure:"encoding"`
}

func Load() (*AppConfig, error) {
	if f, err := os.Open(".env"); err == nil {
		_ = gotenv.OverApply(f)
		f.Close()
	}

	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("config/")
	v.AddConfigPath(".")

	v.SetEnvPrefix("GITLENS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Secrets-only keys (no yaml/default); must be explicitly bound so
	// Viper's AutomaticEnv resolves them during Unmarshal.
	v.BindEnv("github.client_id")
	v.BindEnv("github.client_secret")
	v.BindEnv("encryption.key")
	v.BindEnv("database.password")

	// Defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.shutdown_timeout", "30s")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 3306)
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", "5m")
	v.SetDefault("database.migrations_path", "migrations")
	v.SetDefault("jobs.worker_count", 3)
	v.SetDefault("jobs.poll_interval", "5s")
	v.SetDefault("jobs.stale_running_timeout", "30m")
	v.SetDefault("access.rate_limit_per_min", 100)
	v.SetDefault("access.session_duration", "720h")
	v.SetDefault("access.revalidation_stale_after", "1h")
	v.SetDefault("email.provider", "smtp")
	v.SetDefault("email.smtp.port", 1025)
	v.SetDefault("email.digest_cron", "0 8 * * *")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.encoding", "json")
	v.SetDefault("storage.repos_path", "storage/repos")
	v.SetDefault("storage.forks_path", "storage/forks")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg AppConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

func (c *AppConfig) validate() error {
	if c.GitHub.ClientID == "" {
		return fmt.Errorf("GITLENS_GITHUB_CLIENT_ID is required")
	}
	if c.GitHub.ClientSecret == "" {
		return fmt.Errorf("GITLENS_GITHUB_CLIENT_SECRET is required")
	}
	if c.Encryption.Key == "" {
		return fmt.Errorf("GITLENS_ENCRYPTION_KEY is required")
	}
	if len(c.Encryption.Key) != 32 && len(c.Encryption.Key) != 64 {
		return fmt.Errorf("GITLENS_ENCRYPTION_KEY must be 32 bytes (or 64 hex chars)")
	}
	if c.Database.Password == "" {
		return fmt.Errorf("GITLENS_DATABASE_PASSWORD is required")
	}
	return nil
}
