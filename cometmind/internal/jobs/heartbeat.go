package jobs

import (
	"context"
	"time"
)

const heartbeatInterval = 5 * time.Minute

func heartbeatSessionOnce(ctx context.Context, svc *Service, sessionID string) {
	if svc == nil || sessionID == "" {
		return
	}
	job, ok, err := svc.JobForSession(ctx, sessionID)
	if err != nil || !ok {
		return
	}
	_ = svc.Heartbeat(ctx, job.ID, sessionID)
}

// StartHeartbeatDuringTurn extends a job lease while its agent turn is running.
func StartHeartbeatDuringTurn(ctx context.Context, svc *Service, sessionID string) func() {
	heartbeatSessionOnce(ctx, svc, sessionID)
	if svc == nil || sessionID == "" {
		return func() {}
	}
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				heartbeatSessionOnce(ctx, svc, sessionID)
			}
		}
	}()
	return func() { close(stop) }
}
