# ADR-001 — Template Engine: Templ

**Status:** Accepted
**Date:** 2026-02-28
**Decides:** Open Decision "Template engine" (Phase 1)
**Supersedes:** —

## Context

The spec locks the frontend to server-rendered HTMX (§5) and defers the template engine choice between Go's stdlib `html/template` and [Templ](https://templ.guide). HTMX shifts rendering work to the server: every user interaction returns an HTML fragment, so the template layer is on the critical path for every page load, every partial swap, and every diff view. The engine must compose well at the fragment level, catch errors early, and stay readable as the surface area grows across review UIs, notification emails, and fork reports.

## Decision

Use **Templ** as the template engine for all server-rendered HTML.

## Options Considered

### html/template (stdlib)

Pros: Zero dependencies, universally known in Go, no build step, battle-tested.

Cons: No compile-time type checking — a misspelled field name or wrong pipeline type is a runtime panic. Template composition (`template`, `block`, `define`) is stringly-typed and becomes unwieldy with dozens of HTMX partials. IDE support limited to basic syntax highlighting; no go-to-definition across template boundaries. Refactoring a struct field requires grep-and-pray across `.gohtml` files.

### Templ

Pros: Templates are Go files that compile to `func(ctx, w) error` — type errors are caught by the compiler. Components are first-class functions: composable, testable, and navigable via standard Go tooling. Excellent HTMX ergonomics (partial fragments are just small components). LSP provides autocomplete, rename, and go-to-definition. Output is automatically HTML-escaped by default.

Cons: Adds a code-generation build step (`templ generate`). Newer ecosystem with fewer Stack Overflow answers. Developers unfamiliar with Templ need a short ramp-up, though the syntax is intentionally close to JSX/Go hybrid.

## Rationale

The HTMX architecture means GitLens Pro will have a large number of small fragments: diff hunks, comment threads, review status badges, notification rows, participant chips, file trees. With `html/template`, each fragment is a stringly-typed `{{template "name" .}}` invocation whose data contract is invisible to the compiler. Templ makes each fragment a typed function — breaking a data contract breaks the build, not the user's browser.

The diff and blame views are the most complex rendering surfaces. They combine syntax-highlighted code, line-number anchors, inline-comment slots, and expandable context — all as composable HTMX targets. Templ's component model maps directly to this structure; `html/template` would require deeply nested `define`/`block` hierarchies.

The code-generation step is a minor cost. It runs in under a second for typical projects and integrates cleanly with `go generate` and CI.

## Consequences

- All `.templ` files live alongside their feature packages (e.g., `internal/review/templates/`).
- `templ generate` is added to the Makefile and CI pipeline before `go build`.
- Email templates also use Templ (rendered to string via `templ.ToGoHTML`), keeping a single template language across HTML and email.
- Developers run `templ generate --watch` during local development for hot-reload.
- The Docker multi-stage build runs `templ generate` in the builder stage.

## References

- Spec §5 Technology Stack: Template Engine — DEFERRED
- Spec §5: Frontend Architecture — LOCKED to server-rendered HTMX
- [templ.guide](https://templ.guide)
