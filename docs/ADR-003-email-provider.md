# ADR-003 — Email Provider Strategy: Interface + SMTP Default, SES Production Path

**Status:** Accepted
**Date:** 2026-02-28
**Decides:** Open Decision "Email delivery provider" (Phase 5)
**Supersedes:** —

## Context

The spec requires transactional email for review notifications (FR-NOTIFY-02), with per-review threading headers (FR-NOTIFY-03), three delivery modes (instant, digest, web_only — FR-NOTIFY-04), retry with exponential backoff (§4.4), access revalidation before send (FR-NOTIFY-09), and private-repo content redaction (FR-NOTIFY-08). The technology stack table lists "SMTP / SES / Postmark / Resend" as deferred. The Docker Compose dev environment already includes optional MailHog for email testing (§14).

The decision is not just which provider to use in production but how to structure the mailer module so the provider is swappable without touching notification logic.

## Decision

Define a **`Sender` interface** in the mailer module. Ship with two implementations: **SMTP** (default, used in dev and simple deployments) and **Amazon SES** (recommended production path). The implementation is selected by configuration at startup.

```go
type Sender interface {
    Send(ctx context.Context, msg *Message) error
}
```

## Options Considered

### SMTP (direct)

Pros: Universal protocol. Works with any MTA — MailHog in dev, Postfix in self-hosted prod, or a relay like SendGrid's SMTP endpoint. Zero vendor dependency. Go's `net/smtp` is stdlib.

Cons: Deliverability is entirely your responsibility (SPF, DKIM, IP reputation, bounce handling). No built-in analytics. Raw SMTP has no webhook for delivery status — you only know "accepted by relay."

### Amazon SES

Pros: High deliverability with managed reputation. Cheap at scale ($0.10/1K emails). Bounce and complaint notifications via SNS. DKIM signing built-in. Go SDK is mature. Fits if the deployment already touches AWS.

Cons: AWS dependency. Requires domain verification and sandbox exit. Slightly more complex local development (though the SMTP interface means dev still uses MailHog).

### Postmark

Pros: Best-in-class deliverability. Transactional-only focus means no shared reputation with marketing senders. Excellent API and Go library. Built-in inbound email parsing (useful if reply-by-email is undeferred later).

Cons: Most expensive option (~$1.25/1K emails). Vendor lock-in on their template system if you use it.

### Resend

Pros: Modern DX, clean API, good Go SDK, React Email templates.

Cons: Youngest service — less track record for deliverability at scale. React Email templates are irrelevant since we render with Templ server-side.

## Rationale

The `Sender` interface is the important decision; the specific provider is a configuration detail. The interface has a single method because the mailer module already handles retry logic, threading headers, access revalidation, and content assembly — the provider only needs to transmit a fully-formed message.

SMTP as the default keeps the dev loop simple (MailHog), supports self-hosted deployments with any MTA, and avoids vendor coupling during early phases. SES is the recommended production upgrade because GitLens Pro is already a Docker-deployed application likely headed for AWS/cloud hosting, and SES provides the deliverability infrastructure (bounce handling, reputation management, DKIM) that raw SMTP does not.

Postmark is a strong alternative if the deployment is provider-agnostic or if reply-by-email (currently deferred — FR-NOTIFY-10) becomes a priority; its inbound parsing would simplify that feature. The interface makes this a configuration swap, not a code change.

## Design

```
mailer module
├── sender.go          // Sender interface
├── smtp_sender.go     // SMTP implementation (net/smtp + STARTTLS)
├── ses_sender.go       // SES implementation (aws-sdk-go-v2)
├── message.go         // Message struct: From, To, Subject, HTML, Text,
│                      //   Headers (threading, List-Unsubscribe)
├── renderer.go        // Templ → HTML string + plain-text fallback
├── worker.go          // NotificationWorker: dequeue, revalidate access,
│                      //   render, send, retry
└── digest.go          // Digest aggregation + scheduling
```

Configuration (config.yaml):

```yaml
email:
  provider: smtp          # smtp | ses
  from: "GitLens Pro <notifications@gitlens.example.com>"
  smtp:
    host: localhost
    port: 1025            # MailHog default
    username: ""
    password: ""
    starttls: false
  ses:
    region: us-east-1
    # Credentials from environment (AWS_ACCESS_KEY_ID, etc.)
```

## Consequences

- The `Sender` interface ships in Phase 1 (mailer module skeleton) even though email delivery is Phase 5 scope.
- MailHog remains in Docker Compose for local email inspection.
- Production deployment docs will include SES domain verification and SNS bounce-topic setup.
- Adding a Postmark or Resend sender later is a single-file implementation of the `Sender` interface plus a config enum variant.
- Threading headers (`In-Reply-To`, `References`, `X-GitLens-Review-ID`) are assembled by the worker, not the sender — they are provider-agnostic.
- Bounce/complaint handling for SES is a future enhancement (SNS webhook endpoint). For SMTP, bounces surface only as `email_state=failed` after retry exhaustion.

## References

- Spec §5: Email Delivery — DEFERRED
- Spec §6.1: mailer module provides Mailer, TemplateRenderer, NotificationWorker
- Spec §14: Docker Compose includes optional MailHog
- FR-NOTIFY-02, FR-NOTIFY-03, FR-NOTIFY-08, FR-NOTIFY-09
- ADR-001 (Templ — email templates use the same engine)
