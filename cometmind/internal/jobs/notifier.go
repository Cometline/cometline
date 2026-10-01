package jobs

import "context"

// Notifier handles outbound job status notifications.
type Notifier struct {
	settings func() Settings
	handlers []NotificationHandler
}

// NotificationHandler receives job lifecycle events for external delivery.
type NotificationHandler interface {
	OnJobEvent(ctx context.Context, job Job, action string, detail string)
}

// NewNotifier builds a notifier with dynamic settings.
func NewNotifier(settingsFn func() Settings) *Notifier {
	if settingsFn == nil {
		settingsFn = DefaultSettings
	}
	return &Notifier{settings: settingsFn}
}

// Register adds a notification handler.
func (n *Notifier) Register(h NotificationHandler) {
	if n == nil || h == nil {
		return
	}
	n.handlers = append(n.handlers, h)
}

func (n *Notifier) emit(ctx context.Context, job Job, action, detail string) {
	if n == nil {
		return
	}
	for _, h := range n.handlers {
		h.OnJobEvent(ctx, job, action, detail)
	}
}
