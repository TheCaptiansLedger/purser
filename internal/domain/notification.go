package domain

// NotificationEventType is the discriminant for NotificationEvent payloads.
type NotificationEventType string

// Scan lifecycle event types emitted to NotificationDispatcher.
const (
	NotifyScanStarted    NotificationEventType = "scan_started"
	NotifyFileDiscovered NotificationEventType = "file_discovered"
	NotifyAutoMatched    NotificationEventType = "auto_matched"
	NotifyUnmatched      NotificationEventType = "unmatched"
	NotifyNewUnmatched   NotificationEventType = "new_unmatched"
	NotifyScanComplete   NotificationEventType = "scan_complete"
	NotifyFileMissing    NotificationEventType = "file_missing"
	NotifyUpgradeQueued  NotificationEventType = "upgrade_queued"
	NotifyUpgradeApplied NotificationEventType = "upgrade_applied"
)

// NotificationEvent is emitted by the scan service to ports.NotificationDispatcher.
// Future adapters (Discord, Slack, Gotify, in-app) all receive this same type.
type NotificationEvent struct {
	Type    NotificationEventType
	Payload any // callers cast using Type as a discriminant
}
