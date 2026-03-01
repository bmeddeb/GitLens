# GitLens Pro — Comprehensive Implementation Plan (All 5 Phases)

## Context

GitLens Pro is a greenfield Go application for deep GitHub repository analysis and code review. The requirements PDF (v1.2), 4 ADRs, schema DDL, state machines, and permission matrix are finalized in `docs/`. No source code exists yet. This plan covers all 5 development phases, producing ~95 source files + 10 migrations + infrastructure.

## Conventions

- Each Go package under `internal/` exposes an `fx.Module` variable in `module.go`
- GORM models in `model.go` within each package
- Interfaces live alongside their primary consumer
- Test files (`_test.go`) sit next to their subject
- Migration SQL in `migrations/` with `up` and `down` variants

---

## PHASE 1: Foundation

**Goal:** FX scaffold, config, DB, auth, jobs, sessions. User can log in via GitHub, see profile, log out.

### Implementation Order

```
go.mod / Makefile / Dockerfile / docker-compose.yml
  → internal/config/        (1 - Viper, AppConfig)
  → internal/logger/        (2 - Zap from config)
  → internal/database/      (3 - GORM pool + golang-migrate)
  → internal/user/          (4 - User model, service, repository)
  → internal/auth/          (5 - OAuth, sessions, middleware, AES-256-GCM encryption)
  → internal/jobs/          (6 - Job model, service, worker scaffold — no real workers yet)
  → internal/http/          (7 - Chi router, middleware, response helpers, handlers)
  → migrations/ 001-004     (8 - users, sessions, repositories, analysis tables, jobs)
  → cmd/server/main.go      (9 - fx.New() with all Phase 1 modules)
```

### Files to Create (~30 files)

**Root:** `go.mod`, `Makefile`, `Dockerfile`, `docker-compose.yml`, `config/config.yaml`, `config/config.example.yaml`

**Entry point:** `cmd/server/main.go` — fx.New() with config, logger, database, user, auth, jobs, http modules

**Modules:**

| Package | Files | Key Types/Exports |
|---------|-------|-------------------|
| `internal/config/` | `config.go`, `module.go`, `config_test.go` | `AppConfig` struct (GitHub, Database, Server, Storage, Encryption, Jobs, Access, Mailer, Log sub-configs). Viper loading: `config/config.yaml` + env overrides (prefix `GITLENS_`). Secrets from env only. Fail fast on missing required values. |
| `internal/logger/` | `logger.go`, `module.go` | `NewLogger(cfg) (*zap.Logger)`. OnStop: `logger.Sync()`. |
| `internal/database/` | `database.go`, `migrate.go`, `module.go`, `database_test.go` | `NewDB(cfg, logger) (*gorm.DB)`. OnStart: open pool + run migrations via golang-migrate `file://migrations`. OnStop: close pool. |
| `internal/user/` | `model.go`, `repository.go`, `service.go`, `module.go`, `*_test.go` | `User` GORM model, `UserRepository` interface (FindByGitHubID, FindByID, Create, Update), `UserService` interface (FindOrCreateByGitHub, GetByID). |
| `internal/auth/` | `oauth.go`, `session.go`, `middleware.go`, `encryption.go`, `model.go`, `module.go`, `*_test.go` | `Session` model, `SessionStore` interface, `OAuthHandler` (login redirect, callback, logout), `SessionMiddleware` (reads cookie → injects User into context → 401 if invalid), `EncryptToken`/`DecryptToken` (AES-256-GCM). OAuth flow: state cookie → GitHub redirect (scope: read:user,user:email,repo) → callback exchanges code → GET /user + /user/emails → FindOrCreateByGitHub → create Session → set signed HttpOnly SameSite=Lax Secure cookie. |
| `internal/jobs/` | `model.go`, `repository.go`, `service.go`, `worker.go`, `module.go`, `*_test.go` | `Job` GORM model, `JobRepository` (Create, FindByID, FindByDedupKey, DequeueNext, ResetStaleRunning), `JobService` (Enqueue with dedup_key=SHA256(type\|repo_id\|fork_id), Get, Cancel—requester only, ListByUser, ListByRepo). OnStart: ResetStaleRunning + start empty worker goroutines. OnStop: signal stop, wait 60s, mark interrupted as pending. |
| `internal/http/` | `router.go`, `middleware.go`, `response.go`, `errors.go`, `module.go` | Chi router with middleware chain: RequestID → RealIP → Zap logging → Recoverer → rate limiter (100 req/min per user, IP for unauthenticated). `SuccessResponse{Data, Meta}`, `ErrorResponse{Error{Code, Message, Status}}`, `EncodeCursor`/`DecodeCursor`. `AppError` typed errors (ErrNotFound, ErrForbidden, ErrAuthRequired, ErrValidation). OnStart: start http.Server. OnStop: graceful 30s shutdown. |
| `internal/http/handlers/` | `auth.go`, `health.go`, `job.go`, `user.go` | Auth endpoints, /healthz, /readyz, job list/get/cancel, /auth/me |

**Migrations (001-004):** Exactly as defined in `docs/erd-and-migrations.md`. All 4 included in Phase 1 because migration 004 (jobs) has FKs to repositories and forks.

### Phase 1 Endpoints

| Method | Path | Auth | Handler |
|--------|------|------|---------|
| GET | /healthz | No | health.Healthz |
| GET | /readyz | No | health.Readyz |
| GET | /auth/github/login | No | auth.Login |
| GET | /auth/github/callback | No | auth.Callback |
| POST | /auth/logout | Yes | auth.Logout |
| GET | /auth/me | Yes | user.Me |
| GET | /api/jobs | Yes | job.ListUserJobs |
| GET | /api/jobs/{jobID} | Yes | job.GetJob |
| POST | /api/jobs/{jobID}/cancel | Yes | job.CancelJob |

### Phase 1 Testing

- **Unit:** config Viper loading, encryption round-trip, session CRUD (mock DB), user service (mock repo), job enqueue/dedup/cancel
- **Integration:** dockertest MySQL — run migrations, verify tables
- **API:** httptest — mock OAuth callback, /healthz, job endpoints
- **FX wiring:** `fxtest.New()` verifies DI graph resolves

---

## PHASE 2: Git Core

**Goal:** Repo management, git operations, commit browsing, blame, diffs, sync pipeline, contributor identity. System becomes functional as a repository analysis tool.

### Implementation Order

```
internal/githubclient/   (1 - GitHubClientFactory, per-token rate tracking)
  → internal/gitclient/  (2 - go-git wrapper: clone, pull, log, blame, diff, merge-base)
  → internal/repository/ (3 - Repo + UserRepo models, services, access validation)
  → internal/analysis/   (4 - CommitAnalyzer + BlameAnalyzer, contributor identity)
  → internal/jobs/ updates (5 - CloneWorker + SyncWorker)
  → internal/http/handlers/ updates (6 - repo + analysis handlers)
```

### Files to Create (~20 files)

| Package | Files | Key Types |
|---------|-------|-----------|
| `internal/githubclient/` | `factory.go`, `ratelimit.go`, `module.go`, `factory_test.go` | `GitHubClientFactory` interface (NewClient, ValidateToken, GetRateLimit). Decrypts user's token → oauth2.TokenSource → github.Client. Rate limits tracked per-token in sync.Map. Remaining < 50 → pause non-critical. 403/429 → sleep until reset. |
| `internal/gitclient/` | `service.go`, `clone.go`, `log.go`, `blame.go`, `diff.go`, `module.go`, `*_test.go` | `GitService` interface (Clone, Pull ff-only with fetch+reset fallback, Log with LogOptions, CommitDetail, Diff, Blame, MergeBase, WalkCommits). Types: `CommitInfo`, `DiffResult{Files[]FileDiff}`, `FileDiff{Hunks[]Hunk}`, `Hunk{Lines[]DiffLine}`, `BlameResult{Lines[]BlameLine}`. Merge commits diff against first parent (FR-COMMIT-05). |
| `internal/repository/` | `model.go`, `repository.go`, `user_repo_repository.go`, `service.go`, `user_repo_service.go`, `module.go`, `*_test.go` | `Repository` + `UserRepository` GORM models. `RepoService` (AddRepo, GetRepo, SyncRepo, RemoveRepo, RecheckAccess, ListUserRepos). `UserRepoService` (ValidateAccess → 403 if suspended, RevalidatePrivateRepos — login sweep, RevalidateSingle — tri-state result). AddRepo flow: validate format → GitHub check → existing+ready=201, else enqueue clone=202. |
| `internal/analysis/` | `commit_analyzer.go`, `blame_analyzer.go`, `model.go`, `repository.go`, `module.go`, `*_test.go` | `CommitAnalyzer` (ProcessNewCommits → upsert contributor_identities/aliases/stats). `BlameAnalyzer` (GetBlame → cache check, on miss compute+compress+store if ≤10K lines and ≤5MB). Identity resolution: lookup by observed_email → link or create. |
| `internal/jobs/` additions | `clone_worker.go`, `sync_worker.go`, `*_test.go` | **CloneWorker:** dequeue clone → set running + cloning → clone to storage/repos/{id}/ → walk all commits → ProcessNewCommits → set ready → mark completed. **SyncWorker (§11.6):** pull ff-only → walk HEAD to last_synced_sha → parse+INSERT commits (idempotent) → incremental stats → update last_synced_at → commit transaction. Blame cache SHA-keyed, not mass-invalidated. |
| `internal/http/handlers/` | `repo.go`, `analysis.go` | Repo CRUD + analysis endpoints |

### Phase 2 Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/repos | List user's repos (?state=active\|suspended\|all) |
| POST | /api/repos | Add repo (201 or 202+Job) |
| GET | /api/repos/{repoID} | Repo detail |
| POST | /api/repos/{repoID}/sync | Trigger sync (202+Job) |
| DELETE | /api/repos/{repoID} | Remove repo |
| POST | /api/repos/{repoID}/recheck-access | Recheck access |
| GET | /api/repos/{repoID}/commits | Paginated commits (filters: author, date, path, keyword) |
| GET | /api/repos/{repoID}/commits/{sha} | Commit detail |
| GET | /api/repos/{repoID}/commits/{sha}/diff | Commit diff (?view=unified\|side-by-side) |
| GET | /api/repos/{repoID}/contributors | Paginated contributors |
| GET | /api/repos/{repoID}/blame | Blame (?path=&ref=) |
| GET | /api/repos/{repoID}/jobs | Repo's jobs |

### Phase 2 Testing

- **Git Ops:** in-memory go-git repos with fixture commit histories
- **Unit:** RepoService (mock GitHubClientFactory/JobService), CommitAnalyzer (mock repo), BlameAnalyzer cache paths
- **Integration:** dockertest — clone worker + sync worker full pipeline
- **API:** httptest — add repo 201/202, list with pagination, diff view modes
- **Access revocation:** mock GitHub 403 → verify suspension

---

## PHASE 3: Fork Analysis

**Goal:** Fork discovery, divergence detection, classification, cherry-pick/rebase detection, cross-fork similarity, fork reports.

### Implementation Order

```
internal/analysis/ updates  (1 - ForkAnalyzer, classifier, cherry-pick, rebase, similarity)
  → internal/jobs/ updates  (2 - ForkAnalysisWorker)
  → internal/http/handlers/ (3 - fork endpoints)
```

### Files to Create (~12 files)

| Package | Files | Key Types |
|---------|-------|-----------|
| `internal/analysis/` additions | `fork_analyzer.go`, `classifier.go`, `cherry_pick.go`, `rebase.go`, `similarity.go`, `levenshtein.go`, `fork_model.go`, `fork_repository.go`, `*_test.go` | `ForkAnalyzer` (DiscoverForks, AnalyzeFork, GenerateReport). `Fork` + `ForkChange` GORM models. **Classification rules (§11.2):** 11 priority-ordered rules, first match wins — conventional prefixes (0.9) → file path patterns (0.7-0.8) → stat heuristics (0.5-0.6) → unknown (0.3). **Divergence (§11.1):** clone fork → add upstream remote → merge-base → walk each side for ahead/behind counts → diverged_at. **Cherry-pick (§11.3):** identical message body or byte-identical patch → is_cherry_pick. **Rebase (§11.4):** >95% Levenshtein on unified diff → is_rebase. **Similarity (§11.5):** same file path + >60% patch overlap → cluster. |
| `internal/jobs/` | `fork_analysis_worker.go`, `*_test.go` | Dequeue fork_analysis → set analyzing → clone fork to storage/forks/{id}/ → AnalyzeFork → store results → set complete |
| `internal/http/handlers/` | `fork.go`, `fork_test.go` | Fork endpoints |

### Phase 3 Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/repos/{repoID}/forks | List forks (paginated, up to 1000) |
| POST | /api/repos/{repoID}/forks/{forkID}/analyze | Trigger deep analysis (202+Job) |
| GET | /api/repos/{repoID}/forks/{forkID}/changes | Fork changes |
| GET | /api/repos/{repoID}/fork-report | Fork evolution report |

### Phase 3 Testing

- **Unit (table-driven):** classifier rules (all 11), cherry-pick match/no-match, rebase >95%/<95%, similarity >60%/<60%
- **Git Ops:** in-memory fork topology repos
- **Integration:** full fork analysis pipeline with dockertest
- **API:** list forks, trigger analysis 202, poll job, get changes, get report

---

## PHASE 4: Review Core

**Goal:** Full code review/audit system — creation, revisions, participants, draft comments, publish batches, blocking reviewers, decisions, diff anchoring, re-anchoring (ADR-004), audit-from-fork.

### Implementation Order

```
migrations 005-009          (1 - review tables, cyclic FK resolution)
  → internal/review/        (2 - models, repository, service, diff service, re-anchoring, status)
  → internal/http/handlers/ (3 - review, participant, comment handlers)
```

### Migrations

`005_reviews`, `006_review_revisions`, `007_reviews_latest_revision_fk` (resolves cyclic FK), `008_review_participants`, `009_review_comments` — all exactly as in `docs/erd-and-migrations.md`.

### Files to Create (~15 files)

| Package | Files | Key Types |
|---------|-------|-----------|
| `internal/review/` | `model.go`, `repository.go`, `service.go`, `participant_service.go`, `comment_service.go`, `diff_service.go`, `reanchor.go`, `status.go`, `module.go`, `*_test.go` | **Models:** `Review` (status DEFAULT 'open' per errata E-001), `ReviewRevision` (immutable, append-only), `ReviewParticipant` (role+state), `ReviewComment` (anchor fields + context_fingerprint + anchor_method). **ReviewService:** CreateReview, GetReview, AddRevision, SubmitDecision, Close/Resolve/Reopen, ListReviews. **`status.go` — RecomputeReviewStatus():** if explicit (draft/resolved/closed) keep it; else derive from participants — any changes_requested → changes_requested; has accepted + all blocking accepted → accepted; else open. **`reanchor.go` — ADR-004 three-phase cascade:** (1) SHA-256 fingerprint sliding window match, (2) line-offset mapping + Levenshtein confidence ≥0.5, (3) orphan. File renames handled via diff rename detection. **Publish batch:** load user's drafts (max 200) → compute fingerprints (CONTEXT_RADIUS=3) → set published + batch UUID → apply decision (comment=no transition per E-003, changes_requested/accepted=update participant+recompute, resolved/closed=set explicit). **AddRevision:** verify author → enforce max 50 → create revision → update latest_revision_id → reset all reviewer/blocking_reviewer states to pending → recompute status → run re-anchoring on previous revision's published inline comments. |
| `internal/http/handlers/` | `review.go`, `participant.go`, `comment.go`, `*_test.go` | Review CRUD, participant management, comment operations |

### Phase 4 Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/repos/{repoID}/reviews | List reviews/audits |
| POST | /api/repos/{repoID}/reviews | Create review/audit |
| GET | /api/reviews/{reviewID} | Detail + latest revision + participants |
| POST | /api/reviews/{reviewID}/revisions | Add revision (author only) |
| GET | /api/reviews/{reviewID}/diff | Diff (?revision=&view=) |
| POST | /api/reviews/{reviewID}/participants | Add participant |
| DELETE | /api/reviews/{reviewID}/participants/{pid} | Remove participant |
| POST | /api/reviews/{reviewID}/comments/drafts | Create/update draft |
| GET | /api/reviews/{reviewID}/comments | List comments (?visibility=) |
| POST | /api/reviews/{reviewID}/comments/publish | Publish drafts + optional decision |
| POST | /api/reviews/{reviewID}/comments/{cid}/resolve | Resolve comment |
| POST | /api/reviews/{reviewID}/comments/{cid}/reopen | Reopen comment |
| POST | /api/reviews/{reviewID}/decision | Submit decision |

### Phase 4 Testing

- **Unit:** `status_test.go` (exhaustive FSM transitions), `reanchor_test.go` (fingerprint match after insertion, after rename, offset fallback pass/fail, deleted file → orphan), participant role enforcement, comment publish batch semantics
- **Integration:** full review lifecycle — create → add revision → add participants → draft → publish with decision → verify status
- **API:** authorization tests — non-author add revision (403), subscriber submit decision (403), author accept own (403)
- **Fixture revisions:** two revisions with line shifts → verify re-anchoring

---

## PHASE 5: Notifications & Hardening

**Goal:** In-app notifications, email delivery, preferences, digests, path-owner rules, nightly purges, load testing.

### Implementation Order

```
migration 010                    (1 - notifications, preferences, path_owner_rules)
  → internal/notification/       (2 - models, service, path-owner evaluation)
  → internal/mailer/             (3 - Sender interface, SMTP/SES, renderer, worker, digest)
  → internal/http/handlers/      (4 - notification, preference, path-owner, admin endpoints)
  → Production hardening         (5 - nightly purges, load tests, Dockerfile tuning)
```

### Files to Create (~18 files)

| Package | Files | Key Types |
|---------|-------|-----------|
| `internal/notification/` | `model.go`, `repository.go`, `service.go`, `preferences.go`, `path_owner_service.go`, `module.go`, `*_test.go` | `Notification`, `NotificationPreference`, `PathOwnerRule` models. `NotificationService` (Notify, NotifyReviewEvent, List, MarkRead, MarkAllRead). **NotifyReviewEvent:** load participants → check prefs per event_type (fallback to `*`) → apply suppress_self → check @mentions (scoped to active access) → create Notification with email_state based on delivery_mode (web_only→not_applicable, instant→pending, digest→not_applicable). **PathOwnerService:** ListRules, CreateRule (target must have active access, else 422), DeleteRule, EvaluateRules (match changed paths against path_glob + file_ext filter, skip suspended users, de-dupe by user_id). Event types: review.created, revision_added, reviewer_added, subscriber_added, batch_published, mention, changes_requested, accepted, resolved, closed. |
| `internal/mailer/` | `sender.go`, `smtp_sender.go`, `ses_sender.go`, `message.go`, `renderer.go`, `worker.go`, `digest.go`, `module.go`, `*_test.go`, `templates/*.templ` | `Sender` interface (`Send(ctx, *Message) error`). `Message{From, To, Subject, HTML, Text, Headers}`. **NotificationWorker loop:** poll pending email_state → revalidate access (FR-NOTIFY-09, denied→suppressed) → redact private repo diff hunks (FR-NOTIFY-08) → render Templ → set threading headers (In-Reply-To, References, X-GitLens-Review-ID) → Send → on success: sent, on failure: increment attempts, ≥3 → failed, else retry with exponential backoff (1m, 5m, 25m). **Digest scheduler:** cron (default 0 8 * * *) → aggregate unread for digest-mode users → render+send. OnStart: start worker + digest scheduler. OnStop: flush pending 30s. |
| `internal/http/handlers/` | `notification.go`, `preference.go`, `path_owner.go`, `admin.go`, `*_test.go` | Notification, preference, path-owner, admin diagnostics endpoints |
| `scripts/` | `load_test.sh`, `seed.go` | Load testing + seed data |

### Phase 5 Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/notifications | List user's notifications |
| POST | /api/notifications/{id}/read | Mark read |
| POST | /api/notifications/read-all | Mark all read |
| GET | /api/preferences/notifications | Get prefs |
| PUT | /api/preferences/notifications | Upsert prefs |
| GET | /api/repos/{repoID}/path-owners | List rules |
| POST | /api/repos/{repoID}/path-owners | Create rule |
| DELETE | /api/repos/{repoID}/path-owners/{ruleID} | Delete rule |
| GET | /api/admin/diagnostics | Admin-only diagnostics |

### Production Hardening

**Nightly purges:** notifications (90d), jobs (90d), blame cache (30d stale), orphaned repos (zero user_repositories), digest artifacts (30d).

**Load test targets:** commit list <200ms p95, single diff <500ms p95, blame hit <100ms p95, review+diff <700ms p95, page load <1s p95.

### Phase 5 Testing

- **Unit:** notification service (suppress_self, mention scoping), path-owner glob matching (table-driven), preferences wildcard fallback, worker retry/backoff/suppression/redaction logic, digest aggregation
- **Integration:** full notification flow — create review → add participant → publish → verify notification rows + email_state
- **API:** notification pagination, mark read, preference round-trip, path-owner rule CRUD (invalid target → 422)
- **Email:** MailHog in Docker Compose — verify email arrives with correct threading headers
- **E2E:** Docker Compose full flow: OAuth → add repo → clone → review → notify → email in MailHog
- **Load:** k6 scripts for commit list, diff, blame, review endpoints

---

## Cross-Cutting Patterns

| Pattern | Implementation |
|---------|---------------|
| **Cursor pagination** | Base64-encoded last ID. Repository uses `WHERE id > ? ORDER BY id ASC LIMIT ?+1`. Strip extra row → derive next_cursor. |
| **Rate limiting** | `golang.org/x/time/rate` token bucket per user ID (from session). sync.Map of limiters. IP-based for unauthenticated. |
| **Error handling** | Typed `AppError{Code, Message, Status}`. Handlers check error type → render envelope. Unknown → 500 + log. |
| **Context propagation** | SessionMiddleware injects `User` into context. Services use `UserFromContext(ctx)`. |

## Verification

After each phase:
1. `go build ./...` compiles without errors
2. `go test ./...` passes (unit + integration with dockertest)
3. Docker Compose `docker compose up` starts all services
4. Manual smoke test of new endpoints via curl/httpie
5. FX wiring test (`fxtest.New()`) verifies DI graph resolves

After Phase 5 (complete):
- Full E2E: OAuth → add repo → clone → commits → blame → create review → add reviewer → draft+publish → notification email in MailHog
- Load tests pass performance targets
- Nightly purge runs without error
