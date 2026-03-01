# GitLens Pro — ERD & Migration Plan

**Source:** GitLens_Pro_Requirements_v1_2_Final (v1.2.1 errata applied), ADR-001 through ADR-004, state-machines.md, permission-matrix.md.
**Date:** 2026-02-28
**Engine:** MySQL 8.0+, InnoDB, utf8mb4_unicode_ci.

---

## Schema Decisions Required Before DDL

Both items below were flagged during architecture review and are now resolved.

### 1. Participant `commented` Reset Behavior — RESOLVED

**Issue:** The base spec (§3.8.2) says "Adding a revision resets reviewer and blocking_reviewer states of accepted or changes_requested back to pending," implying `commented` survives a new revision. But participant state is defined as applying to the latest revision only, which makes a carried-over `commented` misleading — the reviewer has not commented on the *current* diff.

**Decision:** All non-watching reviewer/blocking_reviewer states (`pending`, `commented`, `changes_requested`, `accepted`) reset to `pending` on new revision. `watching` (subscriber) is unchanged. Author is unchanged. The state-machines.md appendix has been updated to reflect this.

**Schema impact:** None beyond the existing ENUM. The reset is a service-layer UPDATE, not a schema change.

### 2. ADR-004 Context-Window Persistence — RESOLVED

**Issue:** ADR-004's algorithm text described storing `context_start_line` and `context_end_line` alongside the fingerprint, but the schema impact section only listed `context_fingerprint` and `anchor_method`. These window bounds are trivially recomputed from `(start_line, end_line, CONTEXT_RADIUS)`.

**Decision:** Do not persist window bounds. Only `context_fingerprint` (CHAR(64), SHA-256 hex) and `anchor_method` (ENUM) are stored. Bounds are computed transiently at publish time and at re-anchor time. ADR-004 has been updated to reflect this.

**Schema impact:** ReviewComment gains exactly two columns, not four.

---

## Entity Relationship Diagram

```
┌──────────┐      ┌─────────────────┐      ┌──────────────┐
│  users   │──1:N─│    sessions     │      │  repositories│
└────┬─────┘      └─────────────────┘      └──────┬───────┘
     │                                            │
     │  N:M via user_repositories                 │
     ├────────────────────────────────────────────┤
     │         ┌───────────────────────┐          │
     │         │  user_repositories    │          │
     │         │  (user_id, repo_id)   │          │
     │         │  access_state         │          │
     │         └───────────────────────┘          │
     │                                            │
     │                              ┌─────────────┼────────────────────────────┐
     │                              │             │                            │
     │                         ┌────┴───┐   ┌─────┴──────┐   ┌───────────────┐│
     │                         │ commits│   │   forks    │   │  blame_cache  ││
     │                         └────────┘   └─────┬──────┘   └───────────────┘│
     │                                            │                           │
     │                                      ┌─────┴──────┐                    │
     │                                      │fork_changes│                    │
     │                                      └────────────┘                    │
     │                                                                        │
     │   ┌────────────────────┐                                               │
     │   │contributor_identities│──1:N──┌──────────────────────┐              │
     │   └────────────────────┘        │ contributor_aliases   │              │
     │            │                    └──────────────────────┘              │
     │          1:N                                                          │
     │   ┌────────────────────┐                                              │
     │   │ contributor_stats  │──FK──────────────────────────────────────────┘
     │   └────────────────────┘
     │
     │                              ┌──────────┐
     │                              │   jobs   │──FK→repositories, forks, users
     │                              └──────────┘
     │
     │         ┌──────────────────────────────────────────────────────────┐
     │         │                    REVIEW LAYER                         │
     │         │                                                        │
     │    ┌────┴──────┐       ┌──────────────────┐                      │
     ├──▶ │  reviews  │──1:N──│ review_revisions │                      │
     │    └──┬──┬─────┘       └──────────────────┘                      │
     │       │  │                  ▲                                     │
     │       │  │  latest_revision_id (nullable FK, backfilled)         │
     │       │  │                                                       │
     │       │  ├──1:N──┌──────────────────────┐                        │
     │       │  │       │ review_participants  │                        │
     │       │  │       └──────────────────────┘                        │
     │       │  │                                                       │
     │       │  └──1:N──┌──────────────────────┐                        │
     │       │          │  review_comments     │                        │
     │       │          │  (+ re-anchor cols)  │                        │
     │       │          └──────────────────────┘                        │
     │       │                                                          │
     └───────┼──────────────────────────────────────────────────────────┘
             │
             │    ┌───────────────┐   ┌─────────────────────────┐   ┌──────────────────┐
             ├──▶ │ notifications │   │notification_preferences │   │ path_owner_rules │
             │    └───────────────┘   └─────────────────────────┘   └──────────────────┘
             │
```

### Relationship Summary

| Relationship | Cardinality | Join |
|-------------|------------|------|
| User ↔ Repository | N:M | user_repositories |
| User → Session | 1:N | sessions.user_id |
| Repository → Commit | 1:N | commits.repo_id |
| Repository → Fork | 1:N | forks.source_repo_id |
| Fork → ForkChange | 1:N | fork_changes.fork_id |
| Repository → BlameCache | 1:N | blame_cache.repo_id |
| Repository → Review | 1:N | reviews.repo_id |
| Review → ReviewRevision | 1:N | review_revisions.review_id |
| Review → latest ReviewRevision | 1:1 (nullable) | reviews.latest_revision_id |
| Review ↔ User | N:M | review_participants |
| Review → ReviewComment | 1:N | review_comments.review_id |
| ReviewComment → ReviewComment | self-ref (threading) | review_comments.parent_comment_id |
| User → Notification | 1:N | notifications.user_id |
| Repository → PathOwnerRule | 1:N | path_owner_rules.repo_id |
| Job → Repository, Fork, User | FK refs | jobs.repo_id, fork_id, requested_by |
| ContributorIdentity → ContributorAlias | 1:N | contributor_aliases.identity_id |
| ContributorStat → Repository, ContributorIdentity | FK refs | contributor_stats.repo_id, identity_id |

### Cyclic Dependency: Review ↔ ReviewRevision

`reviews.latest_revision_id` → `review_revisions.id` and `review_revisions.review_id` → `reviews.id` form a cycle. Handled by:

1. Migration 005 creates `reviews` with `latest_revision_id` as a bare BIGINT UNSIGNED NULLABLE (no FK).
2. Migration 006 creates `review_revisions` with FK to `reviews`.
3. Migration 007 adds the FK constraint from `reviews.latest_revision_id` to `review_revisions.id`.

Application-level: `INSERT review` with `latest_revision_id = NULL`, then `INSERT review_revision`, then `UPDATE review SET latest_revision_id = ?`.

---

## DDL — Full Table Definitions

### Migration 001: users, sessions

```sql
-- 001_users_sessions.up.sql

CREATE TABLE users (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    github_id         BIGINT UNSIGNED  NOT NULL,
    username          VARCHAR(255)     NOT NULL,
    display_name      VARCHAR(255)     NULL,
    email             VARCHAR(255)     NULL,
    avatar_url        VARCHAR(512)     NULL,
    access_token      VARBINARY(512)   NOT NULL COMMENT 'AES-256-GCM encrypted',
    token_expiry      DATETIME         NULL,
    role              ENUM('user','admin') NOT NULL DEFAULT 'user',
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    last_login_at     DATETIME         NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_users_github_id (github_id),
    UNIQUE KEY uk_users_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


CREATE TABLE sessions (
    id                CHAR(64)         NOT NULL COMMENT 'Cryptographic random hex',
    user_id           BIGINT UNSIGNED  NOT NULL,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at        DATETIME         NOT NULL,
    last_active_at    DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    INDEX idx_sessions_user_id (user_id),
    INDEX idx_sessions_expires_at (expires_at),

    CONSTRAINT fk_sessions_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**Cascade rule:** Deleting a user cascades to their sessions. User deletion is an admin operation not exposed in v1.2 API.

---

### Migration 002: repositories, user_repositories

```sql
-- 002_repositories.up.sql

CREATE TABLE repositories (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    github_id         BIGINT UNSIGNED  NOT NULL,
    owner             VARCHAR(255)     NOT NULL,
    name              VARCHAR(255)     NOT NULL,
    full_name         VARCHAR(512)     NOT NULL,
    description       TEXT             NULL,
    default_branch    VARCHAR(255)     NOT NULL DEFAULT 'main',
    clone_url         VARCHAR(512)     NOT NULL,
    is_fork           BOOLEAN          NOT NULL DEFAULT FALSE,
    is_private        BOOLEAN          NOT NULL DEFAULT FALSE,
    parent_github_id  BIGINT UNSIGNED  NULL,
    local_path        VARCHAR(512)     NULL,
    clone_status      ENUM('pending','cloning','ready','error')
                                       NOT NULL DEFAULT 'pending'
                                       COMMENT 'Denormalized from Job.status',
    last_synced_at    DATETIME         NULL,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uk_repos_github_id (github_id),
    UNIQUE KEY uk_repos_full_name (full_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


CREATE TABLE user_repositories (
    id                       BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    user_id                  BIGINT UNSIGNED  NOT NULL,
    repo_id                  BIGINT UNSIGNED  NOT NULL,
    added_at                 DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    access_state             ENUM('active','suspended')
                                              NOT NULL DEFAULT 'active',
    last_access_verified_at  DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    nickname                 VARCHAR(255)     NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_user_repos (user_id, repo_id),
    INDEX idx_user_repos_repo_id (repo_id),

    CONSTRAINT fk_user_repos_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_user_repos_repo
        FOREIGN KEY (repo_id) REFERENCES repositories(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**Cascade rules:**
- User deleted → user_repositories rows deleted (user leaves all workspaces).
- Repository deleted → user_repositories rows deleted.
- Canonical repository + clone cleaned up nightly when zero user_repositories remain (§FR-REPO-04). Application-level, not DB cascade.

---

### Migration 003: commits, contributor tables, forks, fork_changes, blame_cache

```sql
-- 003_analysis_tables.up.sql

CREATE TABLE commits (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    repo_id           BIGINT UNSIGNED  NOT NULL,
    sha               CHAR(40)         NOT NULL,
    author_name       VARCHAR(255)     NULL,
    author_email      VARCHAR(255)     NULL,
    committer_name    VARCHAR(255)     NULL,
    committed_at      DATETIME         NOT NULL,
    message           TEXT             NULL,
    files_changed     INT UNSIGNED     NULL,
    insertions        INT UNSIGNED     NULL,
    deletions         INT UNSIGNED     NULL,
    parent_shas       JSON             NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_commits_repo_sha (repo_id, sha),
    INDEX idx_commits_repo_date (repo_id, committed_at DESC),
    INDEX idx_commits_repo_author (repo_id, author_email),

    CONSTRAINT fk_commits_repo
        FOREIGN KEY (repo_id) REFERENCES repositories(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


CREATE TABLE contributor_identities (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    canonical_email   VARCHAR(255)     NOT NULL,
    display_name      VARCHAR(255)     NULL,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uk_contrib_identity_email (canonical_email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


CREATE TABLE contributor_aliases (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    identity_id       BIGINT UNSIGNED  NOT NULL,
    observed_name     VARCHAR(255)     NULL,
    observed_email    VARCHAR(255)     NOT NULL,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uk_contrib_alias_email (observed_email),
    INDEX idx_contrib_alias_identity (identity_id),

    CONSTRAINT fk_contrib_alias_identity
        FOREIGN KEY (identity_id) REFERENCES contributor_identities(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


CREATE TABLE contributor_stats (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    repo_id           BIGINT UNSIGNED  NOT NULL,
    identity_id       BIGINT UNSIGNED  NOT NULL,
    commits_count     INT UNSIGNED     NOT NULL DEFAULT 0,
    insertions        BIGINT UNSIGNED  NOT NULL DEFAULT 0,
    deletions         BIGINT UNSIGNED  NOT NULL DEFAULT 0,
    first_commit_at   DATETIME         NULL,
    last_commit_at    DATETIME         NULL,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uk_contrib_stats (repo_id, identity_id),

    CONSTRAINT fk_contrib_stats_repo
        FOREIGN KEY (repo_id) REFERENCES repositories(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_contrib_stats_identity
        FOREIGN KEY (identity_id) REFERENCES contributor_identities(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


CREATE TABLE forks (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    source_repo_id    BIGINT UNSIGNED  NOT NULL,
    github_fork_id    BIGINT UNSIGNED  NOT NULL,
    fork_owner        VARCHAR(255)     NOT NULL,
    fork_name         VARCHAR(255)     NOT NULL,
    fork_full_name    VARCHAR(512)     NOT NULL,
    clone_url         VARCHAR(512)     NULL,
    local_path        VARCHAR(512)     NULL,
    ahead_by          INT UNSIGNED     NOT NULL DEFAULT 0,
    behind_by         INT UNSIGNED     NOT NULL DEFAULT 0,
    merge_base_sha    CHAR(40)         NULL,
    diverged_at       DATETIME         NULL,
    analysis_status   ENUM('listed','pending','analyzing','complete','error')
                                       NOT NULL DEFAULT 'listed'
                                       COMMENT 'Denormalized from Job.status',
    last_analyzed_at  DATETIME         NULL,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uk_forks_github_id (github_fork_id),
    INDEX idx_forks_source_repo (source_repo_id),

    CONSTRAINT fk_forks_source_repo
        FOREIGN KEY (source_repo_id) REFERENCES repositories(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


CREATE TABLE fork_changes (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    fork_id           BIGINT UNSIGNED  NOT NULL,
    commit_sha        CHAR(40)         NOT NULL,
    file_path         VARCHAR(1024)    NULL,
    change_type       ENUM('feature','bugfix','refactor','docs','chore','unknown')
                                       NOT NULL DEFAULT 'unknown',
    confidence        DECIMAL(3,2)     NOT NULL DEFAULT 0.30,
    is_cherry_pick    BOOLEAN          NOT NULL DEFAULT FALSE,
    is_rebase         BOOLEAN          NOT NULL DEFAULT FALSE,
    upstream_sha      CHAR(40)         NULL COMMENT 'Matched upstream commit if cherry-pick/rebase',
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uk_fork_changes (fork_id, commit_sha),
    INDEX idx_fork_changes_type (fork_id, change_type),

    CONSTRAINT fk_fork_changes_fork
        FOREIGN KEY (fork_id) REFERENCES forks(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


CREATE TABLE blame_cache (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    repo_id           BIGINT UNSIGNED  NOT NULL,
    file_path         VARCHAR(1024)    NOT NULL,
    commit_sha        CHAR(40)         NOT NULL,
    blame_data        MEDIUMBLOB       NOT NULL COMMENT 'Gzip compressed, max 5 MB uncompressed',
    line_count        INT UNSIGNED     NULL,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uk_blame_cache (repo_id, file_path(255), commit_sha),
    INDEX idx_blame_cache_created (created_at),

    CONSTRAINT fk_blame_cache_repo
        FOREIGN KEY (repo_id) REFERENCES repositories(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
    ROW_FORMAT=COMPRESSED;
```

**Notes:**
- `blame_cache` uses `ROW_FORMAT=COMPRESSED` for the MEDIUMBLOB. The file_path key prefix is 255 bytes due to InnoDB index length limits; the full path + sha uniqueness is enforced at the application layer for paths > 255 bytes (rare in practice).
- `fork_changes` cascade-deletes when the parent fork is removed.
- `contributor_identities` are global (not repo-scoped); `contributor_stats` are repo-scoped.

---

### Migration 004: jobs

```sql
-- 004_jobs.up.sql

CREATE TABLE jobs (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    type              ENUM('clone','sync','fork_analysis')
                                       NOT NULL,
    status            ENUM('pending','running','completed','failed','cancelled')
                                       NOT NULL DEFAULT 'pending',
    repo_id           BIGINT UNSIGNED  NOT NULL,
    fork_id           BIGINT UNSIGNED  NULL,
    dedup_key         VARCHAR(255)     NULL COMMENT 'hash(type|repo_id|fork_id); NULL in terminal states',
    requested_by      BIGINT UNSIGNED  NOT NULL,
    progress_pct      TINYINT UNSIGNED NOT NULL DEFAULT 0,
    progress_message  VARCHAR(512)     NULL,
    result_summary    JSON             NULL,
    error_message     TEXT             NULL,
    retry_count       TINYINT UNSIGNED NOT NULL DEFAULT 0,
    max_retries       TINYINT UNSIGNED NOT NULL DEFAULT 3,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at        DATETIME         NULL,
    completed_at      DATETIME         NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_jobs_dedup (dedup_key),
    INDEX idx_jobs_repo_status (repo_id, status),
    INDEX idx_jobs_requested_by (requested_by),
    INDEX idx_jobs_status_created (status, created_at),

    CONSTRAINT fk_jobs_repo
        FOREIGN KEY (repo_id) REFERENCES repositories(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_jobs_fork
        FOREIGN KEY (fork_id) REFERENCES forks(id)
        ON DELETE SET NULL,
    CONSTRAINT fk_jobs_requested_by
        FOREIGN KEY (requested_by) REFERENCES users(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**Dedup mechanics:** `dedup_key` is UNIQUE but NULLABLE. MySQL's UNIQUE index allows multiple NULLs, so terminal jobs (where `dedup_key` is cleared to NULL) don't collide. Enqueue runs in a transaction; on duplicate-key error, the service fetches the existing active job and returns 202 + `X-Job-Deduplicated: true`.

**Cascade rules:**
- Repository deleted → jobs cascade-deleted (historical jobs for a removed repo are not useful).
- Fork deleted → `fork_id` set to NULL (the job record may still be useful for audit).
- User deleted → jobs cascade-deleted.

---

### Migration 005: reviews

```sql
-- 005_reviews.up.sql

CREATE TABLE reviews (
    id                   BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    repo_id              BIGINT UNSIGNED  NOT NULL,
    kind                 ENUM('review','audit')
                                          NOT NULL,
    status               ENUM('draft','open','changes_requested','accepted','resolved','closed')
                                          NOT NULL DEFAULT 'open'
                                          COMMENT 'Denormalized cache; recomputed by service layer',
    author_id            BIGINT UNSIGNED  NOT NULL,
    title                VARCHAR(512)     NOT NULL,
    summary              TEXT             NULL,
    origin_type          ENUM('manual','single_commit','commit_range','fork_change','similarity_cluster')
                                          NOT NULL,
    origin_payload       JSON             NULL,
    latest_revision_id   BIGINT UNSIGNED  NULL COMMENT 'FK added in migration 007',
    created_at           DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    closed_at            DATETIME         NULL,

    PRIMARY KEY (id),
    INDEX idx_reviews_repo_status (repo_id, status, updated_at),
    INDEX idx_reviews_author (author_id, created_at),

    CONSTRAINT fk_reviews_repo
        FOREIGN KEY (repo_id) REFERENCES repositories(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_reviews_author
        FOREIGN KEY (author_id) REFERENCES users(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**`latest_revision_id`** is intentionally created as a bare nullable column here. The FK to `review_revisions` is added in migration 007 after `review_revisions` exists.

**`status` defaults to `'open'`** per errata E-001.

---

### Migration 006: review_revisions

```sql
-- 006_review_revisions.up.sql

CREATE TABLE review_revisions (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    review_id         BIGINT UNSIGNED  NOT NULL,
    revision_number   INT UNSIGNED     NOT NULL,
    base_ref          VARCHAR(255)     NULL,
    head_ref          VARCHAR(255)     NULL,
    base_commit_sha   CHAR(40)         NOT NULL,
    head_commit_sha   CHAR(40)         NOT NULL,
    file_count        INT UNSIGNED     NULL,
    insertions        INT UNSIGNED     NULL,
    deletions         INT UNSIGNED     NULL,
    created_by        BIGINT UNSIGNED  NOT NULL,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uk_revisions_review_num (review_id, revision_number),
    INDEX idx_revisions_created_by (created_by),

    CONSTRAINT fk_revisions_review
        FOREIGN KEY (review_id) REFERENCES reviews(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_revisions_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**Immutability:** Revisions are append-only. The application layer enforces that no UPDATE or DELETE is issued against this table (except cascade). `revision_number` is sequential per review and assigned by the service layer.

---

### Migration 007: reviews.latest_revision_id FK (backfill)

```sql
-- 007_reviews_latest_revision_fk.up.sql

-- Backfill any existing reviews that already have revisions.
-- In a fresh install this is a no-op, but it makes the migration
-- idempotent for dev environments that may have seed data.
UPDATE reviews r
  JOIN (
    SELECT review_id, MAX(id) AS max_rev_id
    FROM review_revisions
    GROUP BY review_id
  ) rr ON r.id = rr.review_id
SET r.latest_revision_id = rr.max_rev_id
WHERE r.latest_revision_id IS NULL;

-- Now add the FK constraint.
ALTER TABLE reviews
    ADD CONSTRAINT fk_reviews_latest_revision
        FOREIGN KEY (latest_revision_id)
        REFERENCES review_revisions(id)
        ON DELETE SET NULL;
```

**ON DELETE SET NULL:** If a review_revision were somehow deleted (only via cascade when the parent review is deleted, which would also delete this row), the FK would not block. In practice the SET NULL path is never hit independently because both tables cascade from the same review.

---

### Migration 008: review_participants

```sql
-- 008_review_participants.up.sql

CREATE TABLE review_participants (
    id             BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    review_id      BIGINT UNSIGNED  NOT NULL,
    user_id        BIGINT UNSIGNED  NOT NULL,
    role           ENUM('author','reviewer','blocking_reviewer','subscriber')
                                    NOT NULL,
    state          ENUM('pending','commented','changes_requested','accepted','watching')
                                    NOT NULL DEFAULT 'pending',
    added_by       BIGINT UNSIGNED  NOT NULL,
    added_reason   ENUM('manual','path_owner_rule','mention','fork_suggestion')
                                    NOT NULL,
    created_at     DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uk_participants_review_user (review_id, user_id),
    INDEX idx_participants_user (user_id),

    CONSTRAINT fk_participants_review
        FOREIGN KEY (review_id) REFERENCES reviews(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_participants_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_participants_added_by
        FOREIGN KEY (added_by) REFERENCES users(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**State semantics:**
- Subscribers always have `state = 'watching'` (set at insert, not mutated by review lifecycle).
- Authors have `role = 'author'` and `state = 'pending'` (never mutated).
- On new revision: service issues `UPDATE review_participants SET state = 'pending', updated_at = NOW() WHERE review_id = ? AND role IN ('reviewer','blocking_reviewer') AND state != 'pending'`.

---

### Migration 009: review_comments

```sql
-- 009_review_comments.up.sql

CREATE TABLE review_comments (
    id                    BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    review_id             BIGINT UNSIGNED  NOT NULL,
    revision_id           BIGINT UNSIGNED  NOT NULL,
    author_id             BIGINT UNSIGNED  NOT NULL,
    parent_comment_id     BIGINT UNSIGNED  NULL,
    publish_batch_id      CHAR(36)         NULL COMMENT 'UUID grouping a publish batch',

    -- Anchor fields (NULL = overall/non-inline comment)
    file_path             VARCHAR(1024)    NULL,
    side                  ENUM('left','right') NULL,
    start_line            INT UNSIGNED     NULL,
    end_line              INT UNSIGNED     NULL,

    body                  TEXT             NOT NULL,
    visibility            ENUM('draft','published')
                                           NOT NULL DEFAULT 'draft',
    is_resolved           BOOLEAN          NOT NULL DEFAULT FALSE,

    -- Re-anchoring fields (ADR-004)
    context_fingerprint   CHAR(64)         NULL COMMENT 'SHA-256 hex of surrounding context; set at publish',
    anchor_method         ENUM('original','fingerprint','offset','orphaned')
                                           NOT NULL DEFAULT 'original',
    orphaned_at           DATETIME         NULL,

    created_at            DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at          DATETIME         NULL,
    resolved_at           DATETIME         NULL,

    PRIMARY KEY (id),
    INDEX idx_comments_review_vis (review_id, visibility, created_at),
    INDEX idx_comments_review_file (review_id, revision_id, file_path(255)),
    INDEX idx_comments_author_draft (author_id, visibility),
    INDEX idx_comments_parent (parent_comment_id),

    CONSTRAINT fk_comments_review
        FOREIGN KEY (review_id) REFERENCES reviews(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_comments_revision
        FOREIGN KEY (revision_id) REFERENCES review_revisions(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_comments_author
        FOREIGN KEY (author_id) REFERENCES users(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_comments_parent
        FOREIGN KEY (parent_comment_id) REFERENCES review_comments(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**Anchor contract (ADR-002):** Inline comments are identified by `(file_path, side, start_line, end_line, revision_id)`. No stored `anchor_id` — the rendered line anchor `F{file}-H{hunk}-{side}{line}` is reproducible from this tuple at render time.

**Re-anchoring (ADR-004):**
- `context_fingerprint`: NULL for drafts and overall comments. Computed and stored at publish time.
- `anchor_method`: `original` on first revision, updated to `fingerprint`, `offset`, or `orphaned` during re-anchor.
- `orphaned_at`: set when re-anchoring fails both fingerprint and offset phases.

**Threading:** `parent_comment_id` self-references for reply threads. CASCADE ensures deleting a parent removes its replies.

---

### Migration 010: notifications, notification_preferences, path_owner_rules

```sql
-- 010_notifications.up.sql

CREATE TABLE notifications (
    id               BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    user_id          BIGINT UNSIGNED  NOT NULL,
    review_id        BIGINT UNSIGNED  NULL,
    comment_id       BIGINT UNSIGNED  NULL,
    event_type       VARCHAR(64)      NOT NULL,
    title            VARCHAR(512)     NOT NULL,
    body_preview     VARCHAR(512)     NULL,
    payload          JSON             NULL,
    is_read          BOOLEAN          NOT NULL DEFAULT FALSE,
    email_state      ENUM('not_applicable','pending','sent','failed','suppressed')
                                      NOT NULL DEFAULT 'not_applicable',
    email_attempts   TINYINT UNSIGNED NOT NULL DEFAULT 0,
    sent_at          DATETIME         NULL,
    read_at          DATETIME         NULL,
    created_at       DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    INDEX idx_notif_user_read (user_id, is_read, created_at),
    INDEX idx_notif_review (review_id, created_at),
    INDEX idx_notif_email_state (email_state, created_at),

    CONSTRAINT fk_notif_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_notif_review
        FOREIGN KEY (review_id) REFERENCES reviews(id)
        ON DELETE SET NULL,
    CONSTRAINT fk_notif_comment
        FOREIGN KEY (comment_id) REFERENCES review_comments(id)
        ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


CREATE TABLE notification_preferences (
    id               BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    user_id          BIGINT UNSIGNED  NOT NULL,
    event_type       VARCHAR(64)      NOT NULL COMMENT '* = default for all events',
    delivery_mode    ENUM('instant','digest','web_only')
                                      NOT NULL,
    suppress_self    BOOLEAN          NOT NULL DEFAULT TRUE,
    created_at       DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uk_notif_prefs (user_id, event_type),

    CONSTRAINT fk_notif_prefs_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


CREATE TABLE path_owner_rules (
    id               BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    repo_id          BIGINT UNSIGNED  NOT NULL,
    path_glob        VARCHAR(512)     NOT NULL,
    file_ext         VARCHAR(32)      NULL,
    target_user_id   BIGINT UNSIGNED  NOT NULL,
    action_role      ENUM('reviewer','blocking_reviewer','subscriber')
                                      NOT NULL,
    is_active        BOOLEAN          NOT NULL DEFAULT TRUE,
    created_by       BIGINT UNSIGNED  NOT NULL,
    created_at       DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    INDEX idx_path_owners_repo (repo_id, is_active),
    INDEX idx_path_owners_target (target_user_id),

    CONSTRAINT fk_path_owners_repo
        FOREIGN KEY (repo_id) REFERENCES repositories(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_path_owners_target
        FOREIGN KEY (target_user_id) REFERENCES users(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_path_owners_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**Notification cascade rules:**
- User deleted → notifications cascade-deleted.
- Review deleted → `review_id` set to NULL (notification remains as historical record).
- Comment deleted → `comment_id` set to NULL.
- 90-day retention enforced by nightly purge job (application-level).

**Path-owner rules:**
- Target user must have active UserRepository access at create time (422 otherwise).
- At evaluation time, rules targeting inactive/suspended users are silently skipped.
- Repository deleted → rules cascade-deleted.

---

## Migration Summary

| # | File | Tables | Dependencies |
|---|------|--------|-------------|
| 001 | `001_users_sessions.up.sql` | `users`, `sessions` | — |
| 002 | `002_repositories.up.sql` | `repositories`, `user_repositories` | users |
| 003 | `003_analysis_tables.up.sql` | `commits`, `contributor_identities`, `contributor_aliases`, `contributor_stats`, `forks`, `fork_changes`, `blame_cache` | repositories |
| 004 | `004_jobs.up.sql` | `jobs` | repositories, forks, users |
| 005 | `005_reviews.up.sql` | `reviews` | repositories, users |
| 006 | `006_review_revisions.up.sql` | `review_revisions` | reviews, users |
| 007 | `007_reviews_latest_revision_fk.up.sql` | (ALTER reviews) | review_revisions |
| 008 | `008_review_participants.up.sql` | `review_participants` | reviews, users |
| 009 | `009_review_comments.up.sql` | `review_comments` | reviews, review_revisions, users |
| 010 | `010_notifications.up.sql` | `notifications`, `notification_preferences`, `path_owner_rules` | users, reviews, review_comments, repositories |

All migrations are forward-only. Down migrations are generated but not expected to be used in production. `golang-migrate v4` manages sequencing and tracks applied migrations in a `schema_migrations` table.

---

## Index Strategy Notes

**Covering indexes for common queries:**

| Query pattern | Index used |
|--------------|-----------|
| List user's active repos | `uk_user_repos (user_id, repo_id)` + filter on `access_state` |
| List reviews for repo by status | `idx_reviews_repo_status (repo_id, status, updated_at)` |
| List user's reviews | `idx_reviews_author (author_id, created_at)` |
| List published comments for review file | `idx_comments_review_file (review_id, revision_id, file_path)` |
| List user's draft comments | `idx_comments_author_draft (author_id, visibility)` |
| Unread notification count | `idx_notif_user_read (user_id, is_read, created_at)` |
| Email worker dequeue | `idx_notif_email_state (email_state, created_at)` |
| Job worker dequeue | `idx_jobs_status_created (status, created_at)` |
| Job dedup check | `uk_jobs_dedup (dedup_key)` |

**No composite index on `(file_path, side, start_line, end_line)`** — anchor lookups during re-anchoring are always scoped by `(review_id, revision_id)` first, then filtered in application code. The `idx_comments_review_file` index covers the initial fan-out.

---

## Cascade & Cleanup Summary

| Parent deleted | Child behavior | Mechanism |
|---------------|---------------|-----------|
| User | Sessions, user_repos, jobs, reviews, participants, comments, notifications, prefs, path_owner_rules | ON DELETE CASCADE |
| Repository | user_repos, commits, forks, blame_cache, jobs, reviews, path_owner_rules | ON DELETE CASCADE |
| Repository (zero refs) | Canonical record + clone deleted | Nightly application job |
| Review | Revisions, participants, comments, notifications.review_id | CASCADE / SET NULL |
| ReviewRevision | Comments on that revision, reviews.latest_revision_id | CASCADE / SET NULL |
| ReviewComment (parent) | Child comments (thread) | ON DELETE CASCADE |
| Fork | fork_changes, jobs.fork_id | CASCADE / SET NULL |
| Notifications | 90-day retention | Nightly application purge |
| Blame cache | Stale entries (unreachable SHA, >30 days) | Nightly application purge |
| Job records | 90-day retention | Nightly application purge |
| Digest artifacts | 30-day retention | Nightly application purge |

---

## Denormalized Fields

| Field | Source of truth | Recomputed on |
|-------|----------------|---------------|
| `repositories.clone_status` | Latest `jobs` row for that repo + type=clone/sync | Job state transition, restart recovery |
| `forks.analysis_status` | Latest `jobs` row for that fork + type=fork_analysis | Job state transition, restart recovery |
| `reviews.status` | Participant states on latest revision + explicit states | Every participant-decision write, new-revision reset, resolve, close, reopen |
