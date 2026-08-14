package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/jobs"
)

func TestStartHeartbeatDuringTurnExtendsLeaseImmediately(t *testing.T) {
	svc, conn := testJobsServiceWithDB(t, nil)
	defer conn.Close()
	ctx := context.Background()

	job, err := svc.Create(ctx, jobs.CreateInput{Description: "heartbeat"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Claim(ctx, job.ID, "session-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `UPDATE jobs SET lease_expires_at = 1, updated_at = 1 WHERE id = ?`, job.ID); err != nil {
		t.Fatal(err)
	}

	stop := jobs.StartHeartbeatDuringTurn(ctx, svc, "session-1")
	stop()
	got, err := svc.Get(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LeaseExpiresAt == nil || *got.LeaseExpiresAt <= time.Now().UnixMilli() {
		t.Fatalf("lease expiry = %v, want a future timestamp", got.LeaseExpiresAt)
	}
}
