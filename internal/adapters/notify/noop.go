package notify

import (
	"context"
	"log/slog"
	"purser/internal/domain"
	"purser/internal/ports"
)

// NoopDispatcher satisfies ports.NotificationDispatcher by logging at debug level.
type NoopDispatcher struct{}

var _ ports.NotificationDispatcher = (*NoopDispatcher)(nil)

// Dispatch logs the event at debug level and returns nil.
func (n *NoopDispatcher) Dispatch(ctx context.Context, e domain.NotificationEvent) error {
	slog.DebugContext(ctx, "notification", "type", e.Type, "payload", e.Payload)
	return nil
}
