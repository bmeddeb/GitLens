# GitLens Pro — State Machines

**Source:** Extracted and normalized from GitLens_Pro_Requirements_v1_2_Final (v1.2.1 errata applied).
**Date:** 2026-02-28

All transitions are authoritative. "→" denotes a valid transition; unlisted transitions are invalid and must return 422.

---

## 1. Review Status

```
                        ┌──────────────────────────────────────────────┐
                        │                                              │
              ┌─────┐   │   ┌──────┐   ┌───────────────────┐   ┌──────┴───┐
  create ───▶│draft│───┼──▶│ open │◀──│changes_requested  │   │ accepted │
              └─────┘   │   └──┬───┘   └─────────┬─────────┘   └──────────┘
                        │      │                  │                  │
                        │      ▼                  ▼                  ▼
                        │   ┌──────────┐   ┌──────────┐      ┌──────────┐
                        │   │ resolved │   │ resolved │      │ resolved │
                        │   └────┬─────┘   └────┬─────┘      └────┬─────┘
                        │        │              │                  │
                        │        ▼              ▼                  ▼
                        │   ┌──────┐                          ┌──────┐
                        └──▶│closed│◀─────────────────────────│closed│
                            └──────┘                          └──────┘
```

### States

| State | Kind | Source of truth |
|-------|------|-----------------|
| `draft` | Explicit | Stored directly. Only valid before first reviewer notification. |
| `open` | Derived | No reviewer/blocking_reviewer in `changes_requested`; acceptance criteria not met. Default on create. |
| `changes_requested` | Derived | Any reviewer or blocking_reviewer on latest revision has state `changes_requested`. |
| `accepted` | Derived | ≥1 reviewer or blocking_reviewer in `accepted`, zero in `changes_requested`, all blocking_reviewers in `accepted`. |
| `resolved` | Explicit | Stored directly. Ends review concern without merge semantics. |
| `closed` | Explicit | Stored directly. Administrative terminal state. |

The `Review.status` column is a **denormalized cache** of the derived value. The service layer recomputes it on every participant-decision write and on new-revision reset (errata E-002).

### Transitions

| From | To | Trigger | Actor |
|------|----|---------|-------|
| (new) | `open` | Create review | Any user with active repo access |
| (new) | `draft` | Create review with draft flag | Any user with active repo access |
| `draft` | `open` | Publish / add first reviewer | Author |
| `open` | `changes_requested` | Participant submits `changes_requested` decision | Reviewer or blocking_reviewer |
| `open` | `accepted` | Participant submits `accepted` and acceptance criteria met | Reviewer or blocking_reviewer |
| `open` | `resolved` | Submit `resolved` decision | Author or blocking_reviewer |
| `open` | `closed` | Submit `closed` decision | Author or blocking_reviewer |
| `changes_requested` | `open` | New revision added (resets participant states) | Author |
| `changes_requested` | `accepted` | Last `changes_requested` reviewer changes to `accepted` and all criteria met | Reviewer or blocking_reviewer |
| `changes_requested` | `resolved` | Submit `resolved` decision | Author or blocking_reviewer |
| `changes_requested` | `closed` | Submit `closed` decision | Author or blocking_reviewer |
| `accepted` | `open` | New revision added (resets participant states) | Author |
| `accepted` | `changes_requested` | Participant submits `changes_requested` decision | Reviewer or blocking_reviewer |
| `accepted` | `resolved` | Submit `resolved` decision | Author or blocking_reviewer |
| `accepted` | `closed` | Submit `closed` decision | Author or blocking_reviewer |
| `resolved` | `open` | Reopen | Author or blocking_reviewer |
| `closed` | `open` | Reopen | Author or blocking_reviewer |

### Acceptance Criteria (computed)

```
accepted =
    ∃ p ∈ participants : p.role ∈ {reviewer, blocking_reviewer} AND p.state = accepted
    AND ∄ p ∈ participants : p.role ∈ {reviewer, blocking_reviewer} AND p.state = changes_requested
    AND ∀ p ∈ participants : p.role = blocking_reviewer → p.state = accepted
```

### Publish-Decision Mapping

| Decision value | Status effect |
|----------------|---------------|
| `comment` | None — publishes batch, no status transition (errata E-003) |
| `changes_requested` | Sets participant state → `changes_requested`; recomputes review status |
| `accepted` | Sets participant state → `accepted`; recomputes review status |
| `resolved` | Sets review status → `resolved` (explicit) |
| `closed` | Sets review status → `closed` (explicit) |

---

## 2. Participant State

```
  add ───▶ pending ───▶ commented
              │              │
              ▼              ▼
        changes_requested ◀──┘
              │
              ▼
          accepted
```

Subscriber role uses `watching` instead of the above states.

### States

| State | Applies to roles | Meaning |
|-------|-----------------|---------|
| `pending` | reviewer, blocking_reviewer | Added but has not acted on latest revision |
| `commented` | reviewer, blocking_reviewer | Published comments but no decision |
| `changes_requested` | reviewer, blocking_reviewer | Submitted changes_requested decision |
| `accepted` | reviewer, blocking_reviewer | Submitted accepted decision |
| `watching` | subscriber | Following updates, no decision rights |

Author role has no mutable state — the author is always present and unchanged.

### Transitions

| From | To | Trigger |
|------|----|---------|
| `pending` | `commented` | Publish batch with `decision=comment` |
| `pending` | `changes_requested` | Publish/decision with `decision=changes_requested` |
| `pending` | `accepted` | Publish/decision with `decision=accepted` |
| `commented` | `changes_requested` | Publish/decision with `decision=changes_requested` |
| `commented` | `accepted` | Publish/decision with `decision=accepted` |
| `changes_requested` | `accepted` | Publish/decision with `decision=accepted` |
| `accepted` | `changes_requested` | Publish/decision with `decision=changes_requested` |
| any of above | `pending` | New revision added (reset) |

Reset rule: Adding a new revision resets all reviewer and blocking_reviewer states (`commented`, `changes_requested`, `accepted`) back to `pending`. Subscribers remain `watching`. Author state is unchanged. Rationale: participant state represents engagement with the *latest* revision; carrying `commented` forward from a prior revision would misrepresent whether the reviewer has seen the current diff.

---

## 3. Job Lifecycle

```
  enqueue ───▶ pending ───▶ running ───▶ completed
                  │            │
                  │            ▼
                  │          failed ───▶ pending  (retry, if retry_count < max_retries)
                  │
                  ▼
              cancelled
```

### States

| State | Meaning |
|-------|---------|
| `pending` | Queued, awaiting worker pickup |
| `running` | Worker executing |
| `completed` | Success. Terminal. |
| `failed` | Error. Terminal unless retry eligible. |
| `cancelled` | User-cancelled. Terminal. |

### Transitions

| From | To | Trigger | Actor |
|------|----|---------|-------|
| (new) | `pending` | Enqueue (POST endpoint) | Any user with repo access |
| `pending` | `running` | Worker dequeue | System |
| `pending` | `cancelled` | Cancel request | Requester only |
| `running` | `completed` | Worker success | System |
| `running` | `failed` | Worker error | System |
| `failed` | `pending` | Auto-retry (retry_count < max_retries) | System |

### Dedup Key Lifecycle

| Job state | `dedup_key` value |
|-----------|-------------------|
| `pending` or `running` | `hash(type\|repo_id\|fork_id)` — UNIQUE index prevents duplicates |
| `completed`, `failed`, `cancelled` | `NULL` — releases the uniqueness slot |

On unique-key collision during enqueue, the service returns the existing active job with HTTP 202 + `X-Job-Deduplicated: true`.

### Denormalized Status Sync

`Repository.clone_status` and `Fork.analysis_status` mirror `Job.status`:

| Job transition | Denormalized field update |
|----------------|--------------------------|
| Job created (pending) | Set to `pending` |
| Job → running | Set to `cloning` / `analyzing` |
| Job → completed | Set to `ready` / `complete` |
| Job → failed | Set to `error` |
| Restart recovery | Reset running jobs to `pending`; reset denormalized fields |

Source of truth is always `Job.status`. Discrepancies corrected from the latest job for each entity.

---

## 4. UserRepository Access State

```
  add ───▶ active ◀───▶ suspended
              │              │
              ▼              ▼
          (removed)      (removed)
```

### States

| State | Meaning |
|-------|---------|
| `active` | User has confirmed GitHub access to this private repo |
| `suspended` | GitHub returned confirmed denial; content blocked |

Public repositories are exempt from revalidation and are always `active`.

### Revalidation Results (Tri-State)

| GitHub check result | Effect on access_state |
|---------------------|----------------------|
| `accessible` | Set `active`, update `last_access_verified_at` |
| `denied` | Set `suspended`, block content access |
| `indeterminate` | No state change; preserve last-known state; surface warning banner on UI |

Indeterminate covers: rate-limit responses, transient network errors, 5xx from GitHub. These never cause suspension.

### Revalidation Schedule

| Trigger | Scope | Behavior |
|---------|-------|----------|
| Login | All private repos in workspace | Full sweep. Accessible → active. Denied → suspended. Indeterminate → no change. |
| Repo-open or review-open | Single repo | Only if `last_access_verified_at` > 1 hour stale. Denied → block + suspend. Indeterminate → warning banner. |
| Sync (POST /sync) | Single repo | Always revalidate. |
| Email send | Single repo per recipient | Always revalidate. Denied → suppress email, suspend. |

### Suspended Repo Behavior

- Excluded from default `GET /api/repos` listing.
- Visible via `GET /api/repos?state=suspended|all`.
- User may trigger `POST /api/repos/:id/recheck-access` to re-verify.
- User may `DELETE /api/repos/:id` to remove association entirely.
- Review/audit objects on suspended repos are inaccessible until access restored.

---

## 5. ReviewComment Visibility

```
  create ───▶ draft ───▶ published
```

| State | Visibility | Transitions to |
|-------|-----------|----------------|
| `draft` | Author only | `published` (via publish batch) |
| `published` | All participants | Terminal (no unpublish) |

Drafts are scoped to the author and invisible to all other users. Publishing is a batch operation that atomically transitions all of the current user's drafts for a review.

---

## 6. Notification Email State

```
  create ───▶ not_applicable
  create ───▶ pending ───▶ sent
                  │
                  ▼
               failed  (after max retries)
  create ───▶ suppressed  (access denied at send time)
```

| State | Meaning |
|-------|---------|
| `not_applicable` | Recipient preference is `web_only` or event doesn't trigger email |
| `pending` | Queued for delivery |
| `sent` | Successfully transmitted to provider |
| `failed` | All retry attempts exhausted |
| `suppressed` | Access revalidation at send time denied recipient access |

Retry: up to 3 attempts with exponential backoff. After exhaustion → `failed`.
