# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GitLens Pro is a multi-user web application for deep analysis and review of GitHub repositories. Users clone repositories locally, inspect commit histories, analyze contributors, run blame, view diffs, analyze fork evolution, and conduct asynchronous code reviews and audits. All classification and review routing is rule-based in v1.2.

Built with Go and Uber FX for dependency injection, backed by MySQL, integrated with the GitHub API and OAuth. The project is in pre-development — the requirements doc (`docs/GitLens_Pro_Requirements_v1.2_Final.pdf`), ADRs, and schema are finalized in `docs/`, but source code has not yet been written.

## Build & Development Commands

```bash
templ generate              # Generate Go code from .templ files (must run before go build)
templ generate --watch      # Watch mode for development hot-reload
go build ./...              # Build the project
go test ./...               # Run all tests
go test ./internal/review/  # Run tests for a specific package
```

Migrations use `golang-migrate v4`. Docker Compose includes App, MySQL, optional Caddy (TLS), and optional MailHog on port 1025 for local email inspection.

## Technology Stack (all LOCKED unless noted)

| Component | Decision |
|-----------|----------|
| Language | Go 1.22+ |
| DI Framework | Uber FX |
| HTTP Router | Chi v5 |
| Database | MySQL 8.0+, InnoDB, utf8mb4_unicode_ci |
| ORM | GORM v2 |
| Git Ops | go-git v5 |
| GitHub API | go-github v60+, x/oauth2 |
| Config | Viper (precedence: env > yaml > defaults; secrets from env only) |
| Logging | Zap |
| Migrations | golang-migrate v4 |
| Frontend | Server-rendered HTMX (no client-side JS framework) |
| Template Engine | Templ (ADR-001, was DEFERRED, now decided) |
| Syntax Highlighting | Chroma (ADR-002, was DEFERRED, now decided) |
| Email | Pluggable `Sender` interface — SMTP default, SES production (ADR-003) |
| Auth | GitHub OAuth 2.0 (Authorization Code flow) |
| Sessions | MySQL sessions table, signed HTTP-only SameSite=Lax cookies. No JWTs. |

## Architecture

### Uber FX Module Structure (§6.1 — authoritative)

| FX Module | Provides | Depends On |
|-----------|----------|------------|
| config | AppConfig | (none) |
| logger | *zap.Logger | AppConfig |
| database | *gorm.DB, migration runner | AppConfig, *zap.Logger |
| auth | OAuthHandler, SessionMiddleware, SessionStore | AppConfig, *gorm.DB, UserService, *zap.Logger |
| user | UserService, UserRepository | *gorm.DB, *zap.Logger |
| repository | RepoService, UserRepoService, RepoRepository | *gorm.DB, *zap.Logger, GitService, GitHubClientFactory |
| gitclient | GitService | AppConfig, *zap.Logger |
| githubclient | GitHubClientFactory | AppConfig, *zap.Logger |
| analysis | CommitAnalyzer, BlameAnalyzer, ForkAnalyzer | GitService, *zap.Logger |
| review | ReviewService, ReviewRepository, ReviewDiffService | *gorm.DB, GitService, RepoService, *zap.Logger |
| notification | NotificationService, NotificationRepository, PathOwnerService | *gorm.DB, ReviewService, RepoService, *zap.Logger |
| mailer | Mailer, TemplateRenderer, NotificationWorker | AppConfig, NotificationService, *zap.Logger |
| jobs | JobService, JobRepository, CloneWorker, AnalysisWorker | *gorm.DB, *zap.Logger, GitService, GitHubClientFactory, RepoService, analysis.* |
| http | (fx.Invoke only — centralized routes) | Chi router, all Service types, SessionMiddleware |

Key design rules:
- **auth depends on UserService** (user module). User create/update called during OAuth flow.
- **githubclient provides GitHubClientFactory**: creates per-user API clients from decrypted tokens. Rate limits tracked per-token, not globally.
- **http module uses fx.Invoke only**: feature modules provide services/handlers; http module invokes route registration. Handler registration centralized in `internal/http/`.
- **jobs limited to user-visible operational jobs**: clone, sync, fork_analysis. Email dispatch uses the notification worker (mailer module), not /api/jobs.

### Directory Layout (§6.5)

| Path | Purpose |
|------|---------|
| `cmd/server/main.go` | FX entry point |
| `internal/config/` | Viper config, AppConfig, validation |
| `internal/database/` | MySQL pool (GORM), migration runner |
| `internal/user/` | User model, UserService, UserRepository |
| `internal/auth/` | OAuth flow, session store, session middleware |
| `internal/repository/` | Repository + UserRepository models, RepoService, UserRepoService |
| `internal/gitclient/` | go-git wrapper: clone, log, blame, diff, merge-base |
| `internal/githubclient/` | GitHubClientFactory, per-user client, rate-limit tracking |
| `internal/analysis/` | CommitAnalyzer, BlameAnalyzer, ForkAnalyzer, classification rules |
| `internal/review/` | Review, Revision, Participant, Comment models, services, diff anchoring |
| `internal/notification/` | Notification, Preference, PathOwnerRule models, services, rule eval |
| `internal/mailer/` | Email rendering, delivery worker, digest scheduler |
| `internal/jobs/` | Job model, JobService, workers |
| `internal/http/` | Chi router, centralized route registration, middleware |
| `internal/http/handlers/` | auth, repo, analysis, review, notification, jobs, admin handlers |
| `migrations/` | Versioned SQL files |
| `config/` | config.yaml, config.example.yaml |
| `storage/repos/` | Repo clones (Docker volume) |
| `storage/forks/` | Fork clones (Docker volume) |

### FX Lifecycle Hooks (§6.2)

**OnStart** (in order): config → logger → database (pool + migrations) → githubclient (validate factory) → jobs (reset stale, start workers) → mailer (start worker + digest scheduler) → http (start Chi server)

**OnStop** (in order): http (graceful 30s drain) → mailer (stop worker, flush 30s) → jobs (signal workers, wait 60s, mark interrupted as pending) → database (close pool) → logger (flush)

### API Design (§8)

Internal JSON REST API. All endpoints except `/auth/*`, `/healthz`, `/readyz` require session cookie.

**Conventions:**
- **Pagination:** cursor-based `?cursor=&limit=` (default 50, max 200). `next_cursor` in response.
- **Sorting:** `?sort=&order=asc|desc`
- **Filtering:** query params per field, multiple values OR'd
- **Async ops:** HTTP 202 + Job body + Location header. Dedup hits return existing job + `X-Job-Deduplicated: true`.
- **Success envelope:** `{ "data": {...}, "meta": { "cursor":..., "next_cursor":..., "limit":... } }`
- **Error envelope:** `{ "error": { "code":"...", "message":"...", "status":... } }`
- Auth failures: 401 `AUTH_REQUIRED` / 403 `FORBIDDEN`

### Key Architectural Decisions (see `docs/ADR-*.md`)

1. **ADR-001 — Templ** over `html/template`: Compile-time type checking, composable components for HTMX fragments. All HTML and email templates use Templ. `.templ` files live alongside feature packages.
2. **ADR-002 — Server-side diff rendering** with Chroma: Diffs fully rendered on server as HTML fragments. Stable line anchor IDs: `F{file_idx}-H{hunk_idx}-{L|R}{line_num}`. Large diffs (>20 files) lazy-loaded via HTMX. Parallel highlighting via bounded worker pool. Target: <700ms p95 for 200-file diffs.
3. **ADR-003 — Email provider interface**: Single `Sender` interface with SMTP and SES implementations. Provider selected by config at startup. MailHog in Docker Compose for local dev.
4. **ADR-004 — Comment re-anchoring**: Three-phase cascade when new revisions are added: (1) SHA-256 context fingerprint match, (2) line-offset with Levenshtein confidence check (>0.5), (3) mark orphaned. Runs synchronously (<200ms for 100 comments).

### State Machines (`docs/state-machines.md`)

Six FSMs govern the system. Key behaviors:
- **Review status** is a denormalized cache recomputed on every participant-decision write and new-revision reset. Default status on create is `open` (not `draft` — errata E-001). `decision=comment` publishes without status transition (errata E-003).
- **Participant states** reset to `pending` on new revision (all non-watching reviewer/blocking_reviewer states, including `commented`).
- **Job dedup**: `dedup_key` is UNIQUE NULLABLE; cleared to NULL in terminal states so MySQL allows re-queuing.
- **UserRepository access**: Tri-state revalidation (accessible/denied/indeterminate). Indeterminate never causes suspension. Public repos exempt.

### Database Schema (`docs/erd-and-migrations.md`)

10 migrations, 19 tables. Key patterns:
- **Cyclic FK** between `reviews.latest_revision_id` ↔ `review_revisions.review_id` resolved across migrations 005→006→007.
- **Cascade rules**: User/repo deletion cascades broadly. Fork deletion sets `jobs.fork_id` to NULL. Review deletion sets `notifications.review_id` to NULL.
- **Nightly purges** (application-level): notifications (90d), jobs (90d), blame cache (stale entries >30d), digest artifacts (30d), orphaned repos (zero user_repositories).

### Permission Model (`docs/permission-matrix.md`)

- All operations require authenticated session.
- Repository content access gated by `user_repositories.access_state` (active/suspended).
- Private repo email notifications redact diff content; access revalidated before send.
- `@mentions` scoped to users with active repository access.
- Subscribers have no decision rights — comment and follow only.
- Blocking reviewers must all be `accepted` for review to reach `accepted` status.

### Fork Analysis Algorithm (§11)

- **Classification** is rule-based, priority-ordered, first match wins. Conventional commit prefixes (feat:, fix:, refactor:, docs:, chore:) have highest confidence (0.9). Heuristic fallbacks based on file patterns and change stats.
- **Cherry-pick detection**: identical message body or byte-identical patch → `is_cherry_pick=true`, excluded from unique count.
- **Rebase detection**: different SHA but >95% patch overlap (Levenshtein on unified diff) → `is_rebase=true`.
- **Similar changes across forks**: unique commits on same file path with >60% patch overlap → "potential duplicate work."
- **Sync pipeline**: git pull → walk new commits → parse + INSERT (idempotent) → incremental ContributorStat update → commit transaction. Blame cache SHA-keyed, not mass-invalidated.

## Security (§12)

- **Token encryption**: AES-256-GCM. Key from env. Rotation = re-encrypt migration.
- **CSRF**: OAuth state param. SameSite=Lax. All mutations POST/PUT/DELETE.
- **Rate limiting**: Per-user API (100 req/min default). GitHub per-token via factory.
- **Input validation**: Repo IDs `^[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$`. GORM parameterized.
- **Clone isolation**: `storage/repos/{id}/`. Server-constructed paths.
- **Secrets**: All from env only.
- **HTTPS**: Behind TLS proxy (Caddy). Secure cookies.

## Testing Strategy (§13)

| Level | Scope | Tools |
|-------|-------|-------|
| Unit | Service/repo logic | testing, testify, mockgen |
| Integration | FX wiring, DB queries | fxtest, dockertest |
| API | HTTP handlers | httptest, chi helpers |
| Git Ops | go-git against fixtures | in-memory repos |
| E2E | OAuth → clone → analyze → review | Docker Compose |

## Development Phases (§15)

1. **Foundation**: FX scaffold, config, DB, auth, jobs, sessions
2. **Git Core**: Clone, commits, blame, diffs, sync pipeline, contributor ID
3. **Fork Analysis**: Fork discovery, divergence, classification, reports
4. **Review Core**: Review/audit model, endpoints, diff anchors, drafts, participants, blocking reviewers, audit-from-fork
5. **Notifications & Hardening**: Email, digests, subscribers, path-owners, load tests, docs

## Non-Goals (v1.2)

- Real-time co-editing or live cursors
- CI/CD integration, build status monitoring, automatic merge gating
- Repository hosting, branch protection, PR orchestration, merge/landing workflows
- Non-GitHub providers (GitLab, Bitbucket)
- AI/ML-powered analysis or summarization
- Public-facing API (REST API is internal contract for HTMX frontend)
- Reply-by-email, inbound email parsing
