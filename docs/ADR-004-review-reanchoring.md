# ADR-004 — Review Re-Anchoring: Context Fingerprint with Line-Offset Fallback

**Status:** Accepted
**Date:** 2026-02-28
**Decides:** Open Decision "Review re-anchoring" (Phase 4)
**Supersedes:** —

## Context

FR-REVIEW-14 requires that when a new revision is added to a review, prior inline comments are re-anchored onto the new diff. Comments that cannot be mapped are marked orphaned but retained. The spec's open-decision table frames this as "strict hunk match vs heuristic" and notes it affects comment mapping accuracy.

Inline comments are stored with `(file_path, side, start_line, end_line, revision_id)` per §7.13. A new revision changes the diff basis (new base/head SHAs), which can shift, modify, or remove the lines a comment was anchored to. The re-anchoring algorithm must handle insertions, deletions, and modifications between revisions while remaining fast enough to run synchronously when a revision is created.

## Decision

Use a **context-fingerprint** strategy as the primary matching method, with **line-offset mapping** as a fast fallback, and **orphan** as the terminal state. Store a short context fingerprint with each inline comment at publish time.

## Options Considered

### A — Line-Number Offset

Compute a diff between the old revision's head and the new revision's head. For each hunk, calculate the line-number shift. Apply the shift to each comment's `start_line`/`end_line`.

Pros: Simple, fast, deterministic. Works well when the commented region is far from any changes between revisions.

Cons: Fragile near edited regions. If lines are inserted or deleted adjacent to or within the commented range, the offset is ambiguous or wrong. No way to distinguish "the line moved" from "the line was replaced by different content."

### B — Context Fingerprint (chosen)

At publish time, store a fingerprint of the content surrounding each comment's anchor: a hash of the N lines centered on the commented range (the "context window"). On re-anchor, scan the new revision's diff for a matching fingerprint. If found, update the line numbers. If not found, fall back to line-offset mapping. If the offset result also fails a confidence check, mark orphaned.

Pros: Robust against insertions/deletions near the comment — the surrounding content is the identity signal. Handles file renames (same content, different path) with an optional filename-agnostic pass. Degrades gracefully: fingerprint → offset → orphan.

Cons: Requires storing additional data per comment (the fingerprint). Slightly more complex implementation. Hash collisions are theoretically possible but vanishingly unlikely with a reasonable window.

### C — Diff-of-Diffs

Compute a three-way diff: old-revision-diff vs new-revision-diff. Map line positions through the resulting transformation.

Pros: Theoretically precise.

Cons: Computationally expensive for large reviews. Semantically confusing (diffing diffs is not a standard operation and edge cases are poorly understood). Implementation complexity is high relative to the accuracy gain over option B.

## Algorithm Detail

### Publish-Time: Store Fingerprint

When a draft comment batch is published, for each inline comment:

1. Compute the context window transiently: `context_start = max(1, start_line - CONTEXT_RADIUS)`, `context_end = end_line + CONTEXT_RADIUS`, where `CONTEXT_RADIUS = 3` (configurable).
2. Read the context lines from the diff's target side (right for additions, left for deletions) at the comment's revision. Normalize by stripping leading/trailing whitespace to tolerate indentation changes.
3. Store `context_fingerprint = SHA-256(normalized_context_lines)` on the ReviewComment row.

The window bounds are **not persisted** — they are cheap to recompute from the comment's `(start_line, end_line)` and the configured radius. Keeping them out of the schema avoids storing data that becomes stale if the radius is tuned later.

### Re-Anchor: Three-Phase Cascade

When `POST /api/reviews/:id/revisions` creates a new revision:

**Phase 1 — Fingerprint Match.** For each prior inline comment on the previous revision:

1. Read the new revision's diff for the same `file_path`.
2. Recompute the expected window size from the comment's line range + `CONTEXT_RADIUS`.
3. Slide a window of that size across the new diff's lines.
4. If a window's SHA-256 matches `context_fingerprint`, record the new `start_line`/`end_line` derived from the window position.
5. On match: create a re-anchored comment reference on the new revision.

**Phase 2 — Line-Offset Fallback.** For comments that did not match in Phase 1:

1. Compute a diff between old revision's `head_commit_sha` and new revision's `head_commit_sha` for the same file.
2. Walk hunks to build a line-number mapping (old line → new line).
3. Apply the mapping to `start_line`/`end_line`.
4. Confidence check: read the mapped lines from the new diff. If the content diverges significantly (Levenshtein ratio < 0.5 against the original context), reject and proceed to Phase 3.
5. On pass: create a re-anchored comment reference on the new revision, flagged as `anchor_method=offset`.

**Phase 3 — Orphan.** Comments that failed both phases are marked `orphaned_at = NOW()`. They remain visible in the UI with a visual indicator ("this comment's original context has changed") and a link to the original revision where the anchor is still valid.

### File Renames

If `file_path` does not exist in the new revision's diff, check the diff's rename detection. If the file was renamed, retry Phases 1–2 against the new path.

### Performance

The algorithm runs synchronously within the `POST /revisions` transaction. For a review with 100 inline comments across 50 files, the expected wall time is <200 ms (dominated by reading the new diff, which is already computed for the revision). The fingerprint comparison is O(comments × new_diff_lines) in the worst case but is bounded by file-path partitioning.

## Schema Impact

Two columns added to `ReviewComment`:

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| context_fingerprint | CHAR(64) | NULLABLE | SHA-256 of surrounding context at publish time |
| anchor_method | ENUM('original','fingerprint','offset','orphaned') | DEFAULT 'original' | How this anchor was established |

`context_fingerprint` is NULL for overall comments (no file anchor) and for draft comments (computed at publish time). `anchor_method` is `original` for the first revision, then updated on re-anchor.

## Consequences

- ReviewComment gains two columns; migration must precede Phase 4 review features.
- The publish-drafts endpoint gains a step: compute and store fingerprints for each inline comment in the batch.
- The add-revision endpoint gains the re-anchor algorithm as a synchronous post-step.
- Orphaned comments are never silently deleted — they remain queryable and visible.
- The `CONTEXT_RADIUS` is a configuration value. Increasing it improves match confidence but reduces tolerance for nearby edits; 3 is a pragmatic default based on typical code review granularity.
- Test fixtures for re-anchoring (§13 "Revision Remap" test scope) must cover: clean offset, fingerprint match after insertion, fingerprint match after rename, offset fallback with confidence pass, offset fallback with confidence fail → orphan, and fully deleted file → orphan.

## References

- Spec §16: Open Decision — "Review re-anchoring: strict hunk match vs heuristic"
- FR-REVIEW-14: New revisions re-anchor prior inline comments; unmappable marked orphaned
- §7.13: ReviewComment schema
- §13: Testing Strategy — "Revision Remap: comment re-anchoring, orphan handling"
- ADR-002 (Diff rendering — anchor ID contract)
