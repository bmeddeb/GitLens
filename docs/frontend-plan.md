# GitLens Pro — Frontend Implementation Plan

## Context

All backend endpoints (86 Go files, 10 migrations, 47 API routes) are implemented and compiling. The frontend needs to be built using server-rendered HTMX with Templ components. No `.templ` files or static assets existed prior to this plan.

**Design decisions:**
- **CSS:** Tailwind CSS (CDN for dev, bundled for production)
- **Style:** Modern dashboard — card-based, generous whitespace, rounded corners, subtle shadows (Linear/Vercel aesthetic)
- **Theme:** Light + dark mode with user toggle (persisted in cookie, respects `prefers-color-scheme` as default)
- **Templates:** Templ components, `.templ` files alongside feature packages
- **Interactivity:** HTMX for partial page updates, no client-side JS framework

## Approach

The frontend is planned in **6 phases**. Each phase we:
1. Discuss the views — layouts, key components, HTMX interactions
2. User approves the design direction
3. Implement

---

## Phase F1: Layout Shell, Auth & Dashboard ✅

**Goal:** Base HTML layout, navigation sidebar, theme toggle, login flow, dashboard landing page.

**Files created:**
- `internal/http/templates/layout.templ` — base HTML (head, Tailwind CDN, HTMX script, dark mode JS)
- `internal/http/templates/nav.templ` — sidebar navigation
- `internal/http/templates/header.templ` — top bar (breadcrumbs, user menu, theme toggle)
- `internal/http/templates/components.templ` — reusable primitives (card, button, badge, pagination, empty state, spinner)
- `internal/http/templates/errors.templ` — 404, 403, 500 error pages
- `internal/http/templates/dashboard.templ` — dashboard page and fragment
- `internal/auth/templates/login.templ` — login page with GitHub OAuth button
- `internal/http/handlers/pages.go` — dashboard + page-level handlers
- `static/favicon.svg`

**Routes:** `GET /` (dashboard), `GET /login`, `GET /fragments/notification-count`

**Key patterns established:**
- `HX-Request` header detection → return fragment (no layout) vs full page
- Dark/light toggle via `class` strategy on `<html>`, cookie-persisted
- Sidebar nav with active state, collapsible on mobile
- HTMX notification badge polling in header (`every 30s`)
- `RequireAuthPage` middleware — redirects to `/login` instead of 401 JSON
- Toast messages via `hx-swap-oob`
- Cursor-based `LoadMore` component for paginated views

---

## Phase F2: Repository Views

**Goal:** Repo list, add repo, repo detail, sync/clone status, jobs list.

**Files to create:**
- `internal/repository/templates/repo_list.templ` — paginated card grid with status badges
- `internal/repository/templates/repo_detail.templ` — repo overview, stats, actions
- `internal/repository/templates/repo_add_modal.templ` — add repo dialog
- `internal/jobs/templates/job_list.templ` — job table with progress
- `internal/jobs/templates/job_row.templ` — HTMX-swappable row for live updates

**Routes:** repo list, repo detail, add repo, sync, remove, recheck access, jobs

**Key interactions:**
- Add repo → HTMX form → shows job progress card with polling
- Clone/sync status badges auto-refresh via `hx-trigger="every 5s"` while in-progress
- Cursor pagination with "Load more" button

---

## Phase F3: Commit & Analysis Views

**Goal:** Commit log, commit detail + diff rendering (unified + side-by-side), blame, contributors.

**Files to create:**
- `internal/analysis/templates/commit_list.templ` — filterable commit log
- `internal/analysis/templates/commit_detail.templ` — commit metadata + diff
- `internal/analysis/templates/contributor_list.templ` — contributor stats table
- `internal/analysis/templates/blame_view.templ` — line-by-line blame
- `internal/http/templates/diff/diff_file_unified.templ` — single-column diff + Chroma
- `internal/http/templates/diff/diff_file_side_by_side.templ` — two-column diff
- `internal/http/templates/diff/diff_hunk.templ` — hunk header + lines
- `internal/http/templates/diff/diff_line.templ` — line with stable anchor ID
- `internal/http/templates/diff/diff_file_header.templ` — collapsible file header
- `internal/http/templates/diff/diff_view_toggle.templ` — unified/side-by-side switcher

**Routes:** commit list, commit detail, commit diff, contributors, blame

**Key design:**
- Diff view toggle is server round-trip (`hx-get` with `?view=` param)
- Files > 20 collapsed, lazy-expanded via HTMX
- Chroma tokenization server-side, CSS classes for theme-aware highlighting
- Stable anchor IDs: `F{file_idx}-H{hunk_idx}-{L|R}{line_num}` (per ADR-002)

---

## Phase F4: Fork Views

**Goal:** Fork listing, analysis trigger, changes, evolution report.

**Files to create:**
- `internal/analysis/templates/fork_list.templ` — forks table with ahead/behind, status
- `internal/analysis/templates/fork_changes.templ` — classified changes
- `internal/analysis/templates/fork_report.templ` — cross-fork summary, similarity clusters

**Routes:** fork list, analyze fork, fork changes, fork report

**Key interactions:**
- "Analyze" button → job created → row polls for status
- Classification badges with confidence scores
- Cherry-pick/rebase indicators
- "Create audit" action links to review creation

---

## Phase F5: Review Views (most complex)

**Goal:** Full code review UI — CRUD, revision diffs with inline comments, draft/publish, participants, decisions.

**Files to create:**
- `internal/review/templates/review_list.templ` — reviews table with status badges
- `internal/review/templates/review_create.templ` — create form
- `internal/review/templates/review_detail.templ` — header + tabs (diff, comments, participants)
- `internal/review/templates/review_diff.templ` — diff with inline comment slots
- `internal/review/templates/review_participants.templ` — participant list, add/remove
- `internal/review/templates/review_comment_thread.templ` — threaded comments
- `internal/review/templates/review_comment_form.templ` — draft comment editor
- `internal/review/templates/review_publish_dialog.templ` — publish + decision selector
- `internal/review/templates/inline_comment_slot.templ` — clickable gutter for inline comments

**Routes:** all review, participant, comment, and decision endpoints

**Key interactions:**
- Click diff gutter → inline comment form (HTMX swap)
- Draft comments yellow-highlighted, visible only to author
- "Publish" opens dialog with decision dropdown
- Re-anchored comments show "outdated" badge
- Orphaned comments grouped separately

---

## Phase F6: Notifications & Settings

**Goal:** Notification feed, preferences, path-owner rules, admin diagnostics.

**Files to create:**
- `internal/notification/templates/notification_list.templ` — feed with read/unread
- `internal/notification/templates/notification_row.templ` — swappable row
- `internal/notification/templates/preference_form.templ` — prefs per event type
- `internal/notification/templates/path_owner_list.templ` — rules table
- `internal/notification/templates/path_owner_form.templ` — add rule form
- `internal/http/templates/admin_diagnostics.templ` — system health (admin only)

**Routes:** notifications, preferences, path-owners, admin diagnostics

---

## Cross-Cutting Patterns

| Pattern | Implementation |
|---------|---------------|
| **Full page vs fragment** | Handler checks `HX-Request` header → fragment only (no layout) or full page |
| **Pagination** | "Load more" button with `hx-get` appending to container |
| **Modals** | `hx-target="#modal"` swaps content into fixed container |
| **Toast messages** | `hx-swap-oob` for flash messages |
| **Loading states** | `hx-indicator` with Tailwind spinner |
| **Dark mode** | `class` on `<html>`, toggled via inline JS, persisted to cookie |
| **Error pages** | 404, 403, 500 Templ templates |

## Verification

After each phase:
1. `templ generate && go build ./...` compiles
2. Manual browser testing of all views
3. HTMX interactions work (partials, swaps, indicators)
4. Dark/light mode renders correctly
5. Responsive at common breakpoints
