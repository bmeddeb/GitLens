# GitLens Pro — Permission Matrix

**Source:** Extracted and normalized from GitLens_Pro_Requirements_v1_2_Final (v1.2.1 errata applied).
**Date:** 2026-02-28

Prerequisite for every operation below: the actor must have an authenticated session (§FR-AUTH-06). Unauthenticated requests return 401.

---

## 1. Repository Operations

All repository operations require that the actor has an **active** UserRepository association unless noted.

| Operation | Endpoint | Permitted actors | Denied | Notes |
|-----------|----------|-----------------|--------|-------|
| List repos | `GET /api/repos` | Any authenticated user | — | Returns only the actor's repos. `?state=` filters. |
| Add repo | `POST /api/repos` | Any authenticated user | — | Creates UserRepository. Validates GitHub access via token. |
| View repo detail | `GET /api/repos/:id` | User with active UserRepository | 403 | Suspended → 403. |
| Sync repo | `POST /api/repos/:id/sync` | User with active UserRepository | 403 | Triggers revalidation. |
| Remove repo | `DELETE /api/repos/:id` | User with UserRepository (any state) | 403 | Deletes the association, not the canonical record. |
| Recheck access | `POST /api/repos/:id/recheck-access` | User with UserRepository (any state) | 403 | Available for suspended repos. |

---

## 2. Review & Audit Operations

Prerequisite: actor must have active UserRepository access for the review's repository. Lost access = lost review access (§Security).

### 2.1 Review Lifecycle

| Operation | Endpoint | Permitted actors | Denied | Spec reference |
|-----------|----------|-----------------|--------|----------------|
| Create review/audit | `POST /api/repos/:id/reviews` | Any user with active repo access | 403 | §3.8.1 |
| View review detail | `GET /api/reviews/:id` | Any user with active repo access | 403 | §FR-REPO-08 |
| Add revision | `POST /api/reviews/:id/revisions` | Review author only | 403 | §3.8.1 |
| View diff | `GET /api/reviews/:id/diff` | Any user with active repo access | 403 | — |
| Submit decision | `POST /api/reviews/:id/decision` | See §2.3 below | 403/422 | §3.8.1, §3.8.2 |

### 2.2 Participant Management

| Operation | Endpoint | Permitted actors | Denied | Notes |
|-----------|----------|-----------------|--------|-------|
| Add reviewer / blocking_reviewer / subscriber | `POST /api/reviews/:id/participants` | Review author | 403 | Target must have active repo access. |
| Self-subscribe | `POST /api/reviews/:id/participants` | Any user with active repo access | — | Adds self as subscriber. |
| Remove participant | `DELETE /api/reviews/:id/participants/:pid` | Review author | 403 | Author cannot be removed. |
| Self-unsubscribe | `DELETE /api/reviews/:id/participants/:pid` | The participant themselves (subscriber only) | 403 | — |

### 2.3 Decision Submission

| Decision | Permitted actors | Constraints |
|----------|-----------------|-------------|
| `comment` | Reviewer, blocking_reviewer | No status transition. |
| `changes_requested` | Reviewer, blocking_reviewer | — |
| `accepted` | Reviewer, blocking_reviewer | Author cannot accept own review. Blocked if any blocking_reviewer is `pending` or `changes_requested`. |
| `resolved` | Author, blocking_reviewer | — |
| `closed` | Author, blocking_reviewer | — |
| reopen (from resolved/closed) | Author, blocking_reviewer | Returns status to `open`. |

Subscribers have **no decision rights**. They may comment and follow updates only.

### 2.4 Comment Operations

| Operation | Endpoint | Permitted actors | Notes |
|-----------|----------|-----------------|-------|
| Create/update draft | `POST /api/reviews/:id/comments/drafts` | Any current participant with active repo access | Drafts visible only to their author. |
| Publish draft batch | `POST /api/reviews/:id/comments/publish` | Draft author only | Publishes that user's drafts as a single batch. |
| Resolve comment | `POST /api/reviews/:id/comments/:cid/resolve` | Comment author, review author, or any blocking_reviewer | — |
| Reopen comment | `POST /api/reviews/:id/comments/:cid/reopen` | Comment author, review author, or any blocking_reviewer | — |
| List comments | `GET /api/reviews/:id/comments` | Any user with active repo access | `?visibility=draft` returns only actor's own drafts. |

---

## 3. Job Operations

| Operation | Endpoint | Permitted actors | Notes |
|-----------|----------|-----------------|-------|
| List own jobs | `GET /api/jobs` | Any authenticated user | Returns `requested_by = current user` only. |
| List repo jobs | `GET /api/repos/:id/jobs` | Any user with repo in workspace | All jobs for that repo. |
| View job | `GET /api/jobs/:id` | Any user with target repo in workspace | — |
| Cancel job | `POST /api/jobs/:id/cancel` | Requester only | Others receive 403. |

Dedup behavior: if a second user triggers the same job (same `dedup_key`), they receive HTTP 202 with the existing job and `X-Job-Deduplicated: true`. This is not a permission denial — it's resource sharing.

---

## 4. Notification & Preference Operations

| Operation | Endpoint | Permitted actors | Notes |
|-----------|----------|-----------------|-------|
| List notifications | `GET /api/notifications` | Any authenticated user | Returns only the actor's notifications. |
| Mark read | `POST /api/notifications/:id/read` | Notification owner | — |
| Mark all read | `POST /api/notifications/read-all` | Any authenticated user | Scoped to actor's notifications. |
| Get preferences | `GET /api/preferences/notifications` | Any authenticated user | — |
| Upsert preferences | `PUT /api/preferences/notifications` | Any authenticated user | — |

---

## 5. Path-Owner Rule Operations

| Operation | Endpoint | Permitted actors | Constraints |
|-----------|----------|-----------------|-------------|
| List rules | `GET /api/repos/:id/path-owners` | Any user with active repo access | — |
| Create rule | `POST /api/repos/:id/path-owners` | Any user with active repo access | Target user must have active UserRepository access; otherwise 422. |
| Delete rule | `DELETE /api/repos/:id/path-owners/:ruleId` | Any user with active repo access | — |

At evaluation time (new revision triggers path-owner rules): rules targeting inactive or suspended users are silently skipped (FR-NOTIFY-11).

---

## 6. Auth & System Endpoints

| Operation | Endpoint | Auth required | Notes |
|-----------|----------|--------------|-------|
| GitHub login | `GET /auth/github/login` | No | Redirects to GitHub. |
| OAuth callback | `GET /auth/github/callback` | No | Creates/updates User, creates Session. |
| Logout | `POST /auth/logout` | Yes | Destroys session, clears cookie. |
| Current user | `GET /auth/me` | Yes | Returns User profile. |
| Health check | `GET /healthz` | No | — |
| Readiness | `GET /readyz` | No | Reports disk space, DB connectivity. |

---

## 7. Cross-Cutting Rules

**Private repo content isolation.** Review, audit, comment, and notification objects inherit repository access. If a user's UserRepository access is suspended, all review operations on that repo return 403 until access is restored.

**Mention scope.** `@mentions` in comments are limited to users with current active repository access. Mentioning a user without access is a no-op (no notification generated).

**Email content redaction.** Private repo email notifications include metadata and deep links but omit raw diff hunks by default (FR-NOTIFY-08). Access is revalidated immediately before email send; if recipient lost access, email is suppressed (FR-NOTIFY-09).

**Self-action suppression.** Users do not receive notifications for their own actions when `suppress_self = true` (default) in their notification preferences (FR-NOTIFY-05).
