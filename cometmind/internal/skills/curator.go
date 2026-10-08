package skills

import "time"

const (
	CuratorStaleAfter   = 14 * 24 * time.Hour
	CuratorArchiveAfter = 30 * 24 * time.Hour
	CuratorDeleteAfter  = 10 * 24 * time.Hour
	CuratorPassInterval = 7 * 24 * time.Hour
	CuratorIdleGate     = 2 * time.Hour

	CuratorStatusActive   = "active"
	CuratorStatusStale    = "stale"
	CuratorStatusArchived = "archived"
)

// CuratorSkill is the persisted lifecycle row the pure transition reads.
type CuratorSkill struct {
	Name             string
	Status           string
	Pinned           bool
	CreatedAt        time.Time
	LastUsedAt       time.Time
	ArchivedAt       time.Time
	DeleteNotifiedAt time.Time
}

// CuratorAction is one deterministic lifecycle change.
type CuratorAction struct {
	Name string
	To   string // stale, archived, deleted
}

// PlanCuratorTransitions returns stale, archive, and delete actions.
// Pinned skills are unchanged. A skill unused for 30 days is archived in the
// same pass that would also mark it stale.
func PlanCuratorTransitions(now time.Time, skills []CuratorSkill) []CuratorAction {
	var actions []CuratorAction
	for _, skill := range skills {
		if skill.Pinned {
			continue
		}
		status := skill.Status
		if status == "" {
			status = CuratorStatusActive
		}
		unused := unusedFor(now, skill)
		if status == CuratorStatusActive && unused >= CuratorStaleAfter {
			status = CuratorStatusStale
			actions = append(actions, CuratorAction{Name: skill.Name, To: CuratorStatusStale})
		}
		if status == CuratorStatusStale && unused >= CuratorArchiveAfter {
			actions = append(actions, CuratorAction{Name: skill.Name, To: CuratorStatusArchived})
			continue
		}
		if status == CuratorStatusArchived && !skill.ArchivedAt.IsZero() && now.Sub(skill.ArchivedAt) >= CuratorDeleteAfter {
			actions = append(actions, CuratorAction{Name: skill.Name, To: "deleted"})
		}
	}
	return actions
}

func unusedFor(now time.Time, skill CuratorSkill) time.Duration {
	base := skill.LastUsedAt
	if base.IsZero() {
		base = skill.CreatedAt
	}
	if base.IsZero() || now.Before(base) {
		return 0
	}
	return now.Sub(base)
}

// CuratorPassDue reports whether a startup or hourly tick should run.
// A missing last pass is the first-install deferral, not a due pass.
func CuratorPassDue(now, lastPass, idleSince time.Time, running bool, startup bool) (run bool, recordFirst bool, noteIdle bool) {
	if running {
		return false, false, false
	}
	if lastPass.IsZero() {
		return false, true, false
	}
	if now.Sub(lastPass) < CuratorPassInterval {
		return false, false, false
	}
	if startup {
		return true, false, false
	}
	if idleSince.IsZero() {
		return false, false, true
	}
	if now.Sub(idleSince) < CuratorIdleGate {
		return false, false, false
	}
	return true, false, false
}
