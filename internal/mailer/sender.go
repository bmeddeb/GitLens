package mailer

import "context"

// Sender is the pluggable email provider interface (ADR-003).
type Sender interface {
	Send(ctx context.Context, msg *Message) error
}
