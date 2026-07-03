package jobs_test

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/jobs"
	_ "modernc.org/sqlite"
)

func testJobsService(t *testing.T) *jobs.Service {
	return testJobsServiceWithSettings(t, nil)
}

func testJobsServiceWithSettings(t *testing.T, settingsFn func() jobs.Settings) *jobs.Service {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	if err := db.EnsureSchema(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	return jobs.NewService(conn, settingsFn, nil)
}

func TestCreateClaimComplete(t *testing.T) {
	svc := testJobsService(t)
	ctx := context.Background()

	job, err := svc.Create(ctx, jobs.CreateInput{
		Description:      "fix CI",
		DefinitionOfDone: "tests pass",
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != jobs.StatusTodo {
		t.Fatalf("status=%s want todo", job.Status)
	}

	claimed, err := svc.Claim(ctx, job.ID, "sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Status != jobs.StatusOngoing || claimed.AssignedSessionID != "sess-1" {
		t.Fatalf("claimed=%+v", claimed)
	}

	_, err = svc.Claim(ctx, job.ID, "sess-2")
	if err != jobs.ErrAlreadyClaimed {
		t.Fatalf("second claim err=%v want ErrAlreadyClaimed", err)
	}

	done, err := svc.Complete(ctx, job.ID, "sess-1", "all green")
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != jobs.StatusDone || done.Progress != "all green" {
		t.Fatalf("done=%+v", done)
	}
}

func TestConcurrentClaimOnlyAssignsOneSession(t *testing.T) {
	svc := testJobsService(t)
	ctx := context.Background()

	job, err := svc.Create(ctx, jobs.CreateInput{Description: "claim race"})
	if err != nil {
		t.Fatal(err)
	}

	const contenders = 8
	start := make(chan struct{})
	type result struct {
		sessionID string
		err       error
	}
	results := make(chan result, contenders)
	var wg sync.WaitGroup
	for i := 0; i < contenders; i++ {
		sessionID := "sess-race-" + string(rune('a'+i))
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.Claim(ctx, job.ID, sessionID)
			results <- result{sessionID: sessionID, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var winner string
	for res := range results {
		if res.err == nil {
			if winner != "" {
				t.Fatalf("multiple successful claims: %s and %s", winner, res.sessionID)
			}
			winner = res.sessionID
			continue
		}
		if res.err != jobs.ErrAlreadyClaimed && res.err != jobs.ErrConflict {
			t.Fatalf("claim error for %s = %v", res.sessionID, res.err)
		}
	}
	if winner == "" {
		t.Fatal("no claim succeeded")
	}

	got, err := svc.Get(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != jobs.StatusOngoing || got.AssignedSessionID != winner {
		t.Fatalf("job=%+v winner=%s", got, winner)
	}
}

func TestReconcileOrphan(t *testing.T) {
	svc := testJobsService(t)
	ctx := context.Background()

	job, err := svc.Create(ctx, jobs.CreateInput{Description: "orphan test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Claim(ctx, job.ID, "sess-orphan"); err != nil {
		t.Fatal(err)
	}

	n, err := svc.Reconcile(ctx, func(sessionID string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("released=%d want 1", n)
	}
	got, err := svc.Get(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != jobs.StatusTodo {
		t.Fatalf("status=%s want todo", got.Status)
	}
}

func TestUpdateTodoOnlyInTodo(t *testing.T) {
	svc := testJobsService(t)
	ctx := context.Background()

	job, err := svc.Create(ctx, jobs.CreateInput{Description: "editable"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateTodo(ctx, job.ID, jobs.UpdateTodoInput{
		Description:      "updated",
		DefinitionOfDone: "done",
	}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Claim(ctx, job.ID, "sess-1"); err != nil {
		t.Fatal(err)
	}
	_, err = svc.UpdateTodo(ctx, job.ID, jobs.UpdateTodoInput{Description: "nope"}, "")
	if err != jobs.ErrNotEditable {
		t.Fatalf("err=%v want ErrNotEditable", err)
	}
}

func TestListIncludeDeleted(t *testing.T) {
	svc := testJobsService(t)
	ctx := context.Background()

	job, err := svc.Create(ctx, jobs.CreateInput{Description: "to archive"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SoftDelete(ctx, job.ID); err != nil {
		t.Fatal(err)
	}

	active, err := svc.List(ctx, jobs.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("active jobs=%d want 0", len(active))
	}

	withDeleted, err := svc.List(ctx, jobs.ListFilter{IncludeDeleted: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(withDeleted) != 1 || withDeleted[0].DeletedAt == nil {
		t.Fatalf("withDeleted=%+v", withDeleted)
	}
}

func TestArchiveCompletedJobsOnly(t *testing.T) {
	svc := testJobsService(t)
	ctx := context.Background()

	job, err := svc.Create(ctx, jobs.CreateInput{Description: "archive me"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Archive(ctx, job.ID); err != jobs.ErrConflict {
		t.Fatalf("archive todo err=%v want ErrConflict", err)
	}
	if _, err := svc.Claim(ctx, job.ID, "sess-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Complete(ctx, job.ID, "sess-1", "done"); err != nil {
		t.Fatal(err)
	}

	archived, err := svc.Archive(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if archived.ArchivedAt == nil {
		t.Fatalf("archived_at nil in %+v", archived)
	}

	active, err := svc.List(ctx, jobs.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("active jobs=%d want 0", len(active))
	}

	withArchived, err := svc.List(ctx, jobs.ListFilter{IncludeArchived: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(withArchived) != 1 || withArchived[0].ArchivedAt == nil {
		t.Fatalf("withArchived=%+v", withArchived)
	}

	unarchived, err := svc.Unarchive(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unarchived.ArchivedAt != nil {
		t.Fatalf("archived_at=%v want nil", unarchived.ArchivedAt)
	}
}

func TestWorkerErrorReleaseSchedulesRetry(t *testing.T) {
	svc := testJobsServiceWithSettings(t, func() jobs.Settings {
		settings := jobs.DefaultSettings()
		settings.MaxConsecutiveFailures = 3
		settings.RetryCooldownMinutes = 5
		settings.MaxRetryCooldownMinutes = 60
		return settings
	})
	ctx := context.Background()

	job, err := svc.Create(ctx, jobs.CreateInput{Description: "retry me"})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := svc.Claim(ctx, job.ID, "sess-1")
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now().Add(4 * time.Minute).UnixMilli()
	failed, err := svc.ReleaseWithClass(ctx, claimed.ID, "sess-1", "worker blew up", jobs.FailureWorkerError)
	if err != nil {
		t.Fatal(err)
	}

	if failed.Status != jobs.StatusTodo || failed.AssignedSessionID != "" || failed.FailureCount != 1 {
		t.Fatalf("failed=%+v, want todo retry job with one failure", failed)
	}
	if failed.NextRetryAt == nil || *failed.NextRetryAt < before {
		t.Fatalf("next_retry_at=%v want scheduled retry", failed.NextRetryAt)
	}
	if failed.LastFailureReason != "worker blew up" {
		t.Fatalf("last_failure_reason=%q", failed.LastFailureReason)
	}
	ready, err := svc.ListReady(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 0 {
		t.Fatalf("ready len=%d want 0 while cooldown is active", len(ready))
	}
}

func TestWorkerErrorReleaseBlocksAtThresholdAndUnblockResets(t *testing.T) {
	svc := testJobsServiceWithSettings(t, func() jobs.Settings {
		settings := jobs.DefaultSettings()
		settings.MaxConsecutiveFailures = 1
		return settings
	})
	ctx := context.Background()

	job, err := svc.Create(ctx, jobs.CreateInput{Description: "block me"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Claim(ctx, job.ID, "sess-1"); err != nil {
		t.Fatal(err)
	}
	blocked, err := svc.ReleaseWithClass(ctx, job.ID, "sess-1", "still broken", jobs.FailureWorkerError)
	if err != nil {
		t.Fatal(err)
	}
	if blocked.Status != jobs.StatusBlocked || blocked.FailureCount != 1 || blocked.NextRetryAt != nil {
		t.Fatalf("blocked=%+v, want blocked without retry schedule", blocked)
	}
	if _, err := svc.Claim(ctx, job.ID, "sess-2"); err != jobs.ErrConflict {
		t.Fatalf("claim blocked err=%v want ErrConflict", err)
	}

	retried, err := svc.Unblock(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retried.Status != jobs.StatusTodo || retried.FailureCount != 0 || retried.NextRetryAt != nil || retried.LastFailureReason != "" {
		t.Fatalf("retried=%+v, want reset todo job", retried)
	}
	if _, err := svc.Claim(ctx, job.ID, "sess-2"); err != nil {
		t.Fatalf("claim after unblock err=%v", err)
	}
}

func TestAgentAndInfraReleaseDoNotIncrementFailures(t *testing.T) {
	svc := testJobsService(t)
	ctx := context.Background()

	agentJob, err := svc.Create(ctx, jobs.CreateInput{Description: "handoff"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Claim(ctx, agentJob.ID, "sess-agent"); err != nil {
		t.Fatal(err)
	}
	released, err := svc.Release(ctx, agentJob.ID, "sess-agent", "handoff")
	if err != nil {
		t.Fatal(err)
	}
	if released.FailureCount != 0 || released.NextRetryAt != nil || released.LastFailureReason != "" {
		t.Fatalf("agent release mutated retry state: %+v", released)
	}

	infraJob, err := svc.Create(ctx, jobs.CreateInput{Description: "orphan"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Claim(ctx, infraJob.ID, "sess-infra"); err != nil {
		t.Fatal(err)
	}
	if n, err := svc.Reconcile(ctx, func(string) bool { return false }); err != nil || n != 1 {
		t.Fatalf("Reconcile() n=%d err=%v want 1 nil", n, err)
	}
	reconciled, err := svc.Get(ctx, infraJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reconciled.FailureCount != 0 || reconciled.NextRetryAt != nil || reconciled.LastFailureReason != "" {
		t.Fatalf("infra release mutated retry state: %+v", reconciled)
	}
}
