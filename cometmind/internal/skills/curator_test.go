package skills

import (
	"testing"
	"time"
)

func TestPlanCuratorTransitions(t *testing.T) {
	now := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	created := now.Add(-40 * 24 * time.Hour)
	tests := []struct {
		name string
		in   CuratorSkill
		want string
	}{
		{name: "fresh stays active", in: CuratorSkill{Name: "a", Status: CuratorStatusActive, CreatedAt: now.Add(-24 * time.Hour)}},
		{name: "14 days becomes stale", in: CuratorSkill{Name: "a", Status: CuratorStatusActive, CreatedAt: now.Add(-14 * 24 * time.Hour)}, want: CuratorStatusStale},
		{name: "30 days archives", in: CuratorSkill{Name: "a", Status: CuratorStatusActive, CreatedAt: created}, want: CuratorStatusArchived},
		{name: "stale at 20 days stays stale", in: CuratorSkill{Name: "a", Status: CuratorStatusStale, CreatedAt: now.Add(-20 * 24 * time.Hour)}, want: ""},
		{name: "pinned is unchanged", in: CuratorSkill{Name: "a", Status: CuratorStatusActive, Pinned: true, CreatedAt: created}},
		{name: "use resets the clock", in: CuratorSkill{Name: "a", Status: CuratorStatusStale, CreatedAt: created, LastUsedAt: now.Add(-24 * time.Hour)}},
		{
			name: "archived 10 days deletes",
			in:   CuratorSkill{Name: "a", Status: CuratorStatusArchived, ArchivedAt: now.Add(-10 * 24 * time.Hour), CreatedAt: created},
			want: "deleted",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlanCuratorTransitions(now, []CuratorSkill{tt.in})
			if tt.want == "" {
				if len(got) != 0 {
					t.Fatalf("actions = %+v", got)
				}
				return
			}
			if len(got) == 0 || got[len(got)-1].To != tt.want {
				t.Fatalf("actions = %+v, want %s", got, tt.want)
			}
		})
	}
}

func TestCuratorPassDue(t *testing.T) {
	now := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	run, first, note := CuratorPassDue(now, time.Time{}, time.Time{}, false, true)
	if run || !first || note {
		t.Fatalf("first install run=%v first=%v note=%v", run, first, note)
	}
	run, _, _ = CuratorPassDue(now, now.Add(-8*24*time.Hour), time.Time{}, true, true)
	if run {
		t.Fatal("running turn should skip")
	}
	run, _, _ = CuratorPassDue(now, now.Add(-8*24*time.Hour), time.Time{}, false, true)
	if !run {
		t.Fatal("startup after 7 days should run")
	}
	run, _, note = CuratorPassDue(now, now.Add(-8*24*time.Hour), time.Time{}, false, false)
	if run || !note {
		t.Fatalf("hourly without idle observation run=%v note=%v", run, note)
	}
	run, _, _ = CuratorPassDue(now, now.Add(-8*24*time.Hour), now.Add(-2*time.Hour), false, false)
	if !run {
		t.Fatal("hourly after 2 idle hours should run")
	}
	run, _, _ = CuratorPassDue(now, now.Add(-24*time.Hour), now.Add(-3*time.Hour), false, false)
	if run {
		t.Fatal("hourly before 7 days should not run")
	}
}
