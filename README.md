# GitLens Pro

A multi-user web application for deep analysis and review of GitHub repositories. Clone repos locally, inspect commit histories, analyze contributors, run blame, view diffs, analyze fork evolution, and conduct asynchronous code reviews and audits.

## Tech Stack

| Component | Choice |
|-----------|--------|
| Language | Go 1.22+ |
| DI | Uber FX |
| Router | Chi v5 |
| Database | MySQL 8.0 (GORM v2) |
| Git | go-git v5 |
| GitHub API | go-github v60, x/oauth2 |
| Config | Viper |
| Logging | Zap |
| Migrations | golang-migrate v4 |
| Templates | Templ + HTMX |
| Syntax Highlighting | Chroma |
| Email | SMTP / AWS SES |

## Prerequisites

- Go 1.22+
- Docker & Docker Compose
- [templ](https://templ.guide) CLI (`go install github.com/a-h/templ/cmd/templ@latest`)
- A [GitHub OAuth App](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/creating-an-oauth-app) (callback URL: `http://localhost:8080/auth/github/callback`)

## Quick Start

```bash
# 1. Configure environment
cp .env.example .env
# Edit .env with your GitHub OAuth credentials and a generated encryption key:
#   GITLENS_GITHUB_CLIENT_ID=...
#   GITLENS_GITHUB_CLIENT_SECRET=...
#   GITLENS_ENCRYPTION_KEY=$(openssl rand -hex 32)

# 2. (Optional) Customize app settings
cp config/config.example.yaml config/config.yaml

# 3. Start services
docker compose up -d

# 4. App is running at http://localhost:8080
#    MailHog UI at http://localhost:8025
```

### Local Development (without Docker)

```bash
# Start MySQL (via Docker or locally), then:
cp .env.example .env
# Edit .env with your values, then source it:
set -a && source .env && set +a

make run
```

## Build & Development

```bash
make build              # Build binary (runs templ generate first)
make run                # Run the server
make test               # Run all tests with race detector
make test-cover         # Tests with HTML coverage report
make generate           # Generate Go code from .templ files
make generate-watch     # Watch mode for template hot-reload
make lint               # Run golangci-lint
make docker-up          # Start Docker Compose stack
make docker-down        # Stop Docker Compose stack
```

## Database Migrations

Migrations run automatically on startup. To run manually:

```bash
export GITLENS_DATABASE_DSN="gitlens:password@tcp(localhost:3306)/gitlens"
make migrate-up         # Apply all pending migrations
make migrate-down       # Roll back one migration
```

10 migrations cover: users/sessions, repositories, analysis tables, jobs, reviews, revisions, participants, comments, and notifications.

## Configuration

Configuration uses Viper with precedence: **environment variables > config.yaml > defaults**.

All secrets must be set via environment variables with the `GITLENS_` prefix:

| Variable | Description |
|----------|-------------|
| `GITLENS_GITHUB_CLIENT_ID` | GitHub OAuth app client ID |
| `GITLENS_GITHUB_CLIENT_SECRET` | GitHub OAuth app client secret |
| `GITLENS_ENCRYPTION_KEY` | 64 hex char (32-byte) key for AES-256-GCM token encryption |
| `GITLENS_DATABASE_PASSWORD` | MySQL password |

See `config/config.example.yaml` for all available settings.

## Project Structure

```
cmd/server/             Entry point (fx.New)
internal/
  config/               Viper config loading, AppConfig struct
  logger/               Zap logger setup
  database/             GORM connection pool, migration runner
  user/                 User model, service, repository
  auth/                 GitHub OAuth, sessions, middleware, AES-256-GCM encryption
  githubclient/         Per-user GitHub API client factory, rate-limit tracking
  gitclient/            go-git wrapper (clone, pull, log, blame, diff, merge-base)
  repository/           Repository + UserRepository models, services, access validation
  analysis/             Commit analysis, blame caching, fork analysis, classification
  review/               Code review system (reviews, revisions, participants, comments)
  notification/         Notifications, preferences, path-owner rules
  mailer/               Email rendering + delivery (SMTP/SES), digest scheduler
  jobs/                 Background job queue, workers (clone, sync, fork analysis)
  http/                 Chi router, middleware, response helpers
  http/handlers/        HTTP handlers + route registration
migrations/             SQL migration files (up + down)
config/                 YAML configuration files
storage/repos/          Cloned repositories (Docker volume)
storage/forks/          Cloned forks (Docker volume)
```

## API Endpoints

All endpoints except auth and health require an authenticated session cookie.

### Auth & Health
| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Liveness check |
| GET | `/readyz` | Readiness check (DB ping) |
| GET | `/auth/github/login` | Start GitHub OAuth flow |
| GET | `/auth/github/callback` | OAuth callback |
| POST | `/auth/logout` | End session |
| GET | `/auth/me` | Current user profile |

### Repositories
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/repos` | List user's repos |
| POST | `/api/repos` | Add repo (201 or 202 + job) |
| GET | `/api/repos/{repoID}` | Repo detail |
| POST | `/api/repos/{repoID}/sync` | Trigger sync |
| DELETE | `/api/repos/{repoID}` | Remove repo |
| POST | `/api/repos/{repoID}/recheck-access` | Recheck GitHub access |

### Analysis
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/repos/{repoID}/commits` | Paginated commits |
| GET | `/api/repos/{repoID}/commits/{sha}` | Commit detail |
| GET | `/api/repos/{repoID}/commits/{sha}/diff` | Commit diff |
| GET | `/api/repos/{repoID}/contributors` | Contributors |
| GET | `/api/repos/{repoID}/blame` | Blame |

### Forks
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/repos/{repoID}/forks` | List forks |
| POST | `/api/repos/{repoID}/forks/{forkID}/analyze` | Trigger analysis |
| GET | `/api/repos/{repoID}/forks/{forkID}/changes` | Fork changes |
| GET | `/api/repos/{repoID}/fork-report` | Fork evolution report |

### Reviews
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/repos/{repoID}/reviews` | List reviews |
| POST | `/api/repos/{repoID}/reviews` | Create review |
| GET | `/api/reviews/{reviewID}` | Review detail |
| POST | `/api/reviews/{reviewID}/revisions` | Add revision |
| GET | `/api/reviews/{reviewID}/diff` | Review diff |
| POST | `/api/reviews/{reviewID}/participants` | Add participant |
| DELETE | `/api/reviews/{reviewID}/participants/{pid}` | Remove participant |
| POST | `/api/reviews/{reviewID}/comments/drafts` | Create draft comment |
| GET | `/api/reviews/{reviewID}/comments` | List comments |
| POST | `/api/reviews/{reviewID}/comments/publish` | Publish drafts |
| POST | `/api/reviews/{reviewID}/decision` | Submit decision |

### Notifications & Preferences
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/notifications` | List notifications |
| POST | `/api/notifications/{id}/read` | Mark read |
| POST | `/api/notifications/read-all` | Mark all read |
| GET | `/api/preferences/notifications` | Get preferences |
| PUT | `/api/preferences/notifications` | Update preferences |
| GET | `/api/repos/{repoID}/path-owners` | List path-owner rules |
| POST | `/api/repos/{repoID}/path-owners` | Create rule |
| DELETE | `/api/repos/{repoID}/path-owners/{ruleID}` | Delete rule |

### API Conventions

- **Pagination:** cursor-based `?cursor=&limit=` (default 50, max 200)
- **Async operations:** HTTP 202 + Job body + `Location` header
- **Success:** `{ "data": {...}, "meta": { "next_cursor": "...", "limit": 50 } }`
- **Error:** `{ "error": { "code": "...", "message": "...", "status": 400 } }`

## Architecture Decisions

See `docs/ADR-*.md` for details:

1. **ADR-001** — Templ for type-safe, composable HTML templates
2. **ADR-002** — Server-side diff rendering with Chroma syntax highlighting
3. **ADR-003** — Pluggable email provider (SMTP default, SES for production)
4. **ADR-004** — Three-phase comment re-anchoring on new revisions

## License

Proprietary. All rights reserved.
