# ADR-002 — Diff Rendering: Server-Side HTML with Chroma

**Status:** Accepted
**Date:** 2026-02-28
**Decides:** Open Decisions "Diff rendering" and "Syntax highlighting" (Phase 2)
**Supersedes:** —

## Context

GitLens Pro renders diffs in four contexts: single-commit detail (FR-COMMIT-03), review/audit diff (FR-REVIEW-05), fork comparison (FR-FORK-04), and blame overlay (FR-BLAME-01). All four must expose stable file/hunk/line anchors for inline review comments (FR-COMMIT-06, FR-REVIEW-06) and support unified and side-by-side views with expandable context (FR-COMMIT-04). The spec also sets a <700 ms p95 target for review diffs up to 200 files.

The frontend is locked to server-rendered HTMX, so the fundamental question is whether the diff HTML is assembled on the server or whether the server sends structured data and a client-side JS library renders it.

## Decision

Render diff HTML **entirely on the server**. Use go-git's diff primitives for structural parsing, **Chroma** for syntax highlighting, and Templ components for the HTML. The server returns ready-to-insert HTML fragments, one per file, streamed as an HTMX multi-swap or lazy-loaded per file tab.

## Options Considered

### A — Server-side HTML (Chroma)

The server parses the diff, tokenizes each side through Chroma, and emits complete HTML with stable `id` attributes on every line row. HTMX swaps fragments directly into the DOM.

Pros: Single rendering path consistent with the HTMX architecture. Anchors are authoritative (server-generated, never client-reconstructed). Chroma is pure Go — no subprocess, no JS runtime. Diff rendering is testable with standard Go test fixtures. Side-by-side layout is just a different Templ component over the same parsed data.

Cons: Larger HTML payloads than raw JSON. Server bears all CPU cost for highlighting. Adding a new language requires a Chroma lexer (coverage is broad but not universal).

### B — Client-side JS (highlight.js or Monaco)

The server returns a structured JSON diff (file paths, hunks, lines, raw text). A client-side library renders the table and applies syntax highlighting.

Pros: Smaller wire payloads. Rich client interactions (inline editing, real-time cursor) are easier. highlight.js has very broad language coverage.

Cons: Contradicts the HTMX architecture — requires a substantial JS application layer for rendering, anchor generation, and view switching. Anchors become client-derived, creating a mismatch risk between what the client renders and what the server expects when saving inline comments. Two rendering paths (server for page-load, client for interaction) is a common source of bugs.

### C — Hybrid (server structure + client highlighting)

Server returns HTML with unhighlighted `<code>` blocks; client applies highlighting via highlight.js after insertion.

Pros: Smaller initial payload; progressive enhancement.

Cons: Flash of unhighlighted content. Still requires a JS highlighting pass on every swap. Anchor generation split across server (line numbers) and client (token offsets), complicating the inline-comment model.

## Rationale

Option A is the natural choice given the locked HTMX architecture. The server already owns the diff computation (go-git), the anchor generation (FR-REVIEW-06), and the HTML response. Adding a client-side rendering layer would duplicate logic and undermine the "server is the source of truth" principle that makes HTMX simple.

The performance concern is manageable. Chroma tokenization is fast (single-digit milliseconds per file for typical source), and the 200-file / <700 ms p95 budget is achievable by highlighting files in parallel with a bounded worker pool. For very large diffs, files beyond a configurable threshold are rendered on-demand via lazy HTMX loads (`hx-trigger="intersect"`), so the initial response only includes the file list and the first N expanded files.

Chroma covers 200+ languages, which exceeds what GitLens Pro is likely to encounter. For edge cases, falling back to plain-text rendering is acceptable.

## Design Sketch

```
go-git diff → []FileDiff{path, hunks[]Hunk{lines[]Line}}
                          ↓
                   Chroma tokenize each line (parallel, pooled)
                          ↓
                   Templ component per file:
                     unified:    single-column <table>, line-number gutters
                     side-by-side: two-column <table>, synced scroll
                          ↓
                   Stable anchor IDs per line:
                     id="F{file_idx}-H{hunk_idx}-{L|R}{line_num}"
                          ↓
                   HTMX response:
                     full page  → complete diff view
                     file swap  → single file fragment (lazy load)
                     view toggle → re-rendered file in alternate layout
```

## Anchor Contract

Every diff line rendered by the server carries a deterministic `id` attribute:

```
F{file_index}-H{hunk_index}-{side}{line_number}
```

Where `side` is `L` (base/left) or `R` (head/right). These anchors are what inline review comments reference. Because they are server-generated and derived from the parsed diff structure, they are stable across re-renders of the same revision and reproducible from the stored `(file_path, side, start_line, end_line)` tuple in ReviewComment.

## Consequences

- Chroma added as a Go dependency. Pinned version in go.mod.
- A `DiffRenderer` service is introduced in the `review` module, shared by commit-detail, review-diff, and fork-comparison handlers.
- Templ components: `DiffFileUnified`, `DiffFileSideBySide`, `DiffHunk`, `DiffLine`, `InlineCommentSlot`.
- Large-diff handling: files beyond `max_expanded_files` (default 20) render as collapsed headers with `hx-get` for on-demand expansion.
- View toggle (unified ↔ side-by-side) is a server round-trip returning the alternate Templ component for the same parsed data. Parsed diff data is cached in-memory for the duration of the request or short-lived (TTL ~60 s) for rapid toggles.
- Syntax highlighting for email notifications is not needed; email diffs use monospace plain-text or minimal inline styles.

## References

- Spec §5: Frontend Architecture — LOCKED to server-rendered HTMX
- Spec §16: Open Decisions — "Diff rendering" and "Syntax highlighting"
- FR-COMMIT-04, FR-COMMIT-06, FR-REVIEW-05, FR-REVIEW-06, FR-REVIEW-07
- [Chroma](https://github.com/alecthomas/chroma)
- ADR-001 (Templ)
