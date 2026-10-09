package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/inbox"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/skillcurator"
	"github.com/Cometline/cometline/cometmind/internal/skills"
	"github.com/Cometline/cometline/cometmind/internal/tools"
	"go.uber.org/zap"
)

// skillCuratorKind distinguishes a merge child from a skill-review child.
const skillCuratorKind = "skill_curator"

// StartSkillCurator runs the hidden self-improvement skill maintenance loop.
func (r *Runtime) StartSkillCurator(ctx context.Context) {
	if r == nil || r.Curator == nil || r.workers == nil {
		return
	}
	r.workers.Go(ctx, func(ctx context.Context) {
		r.curatorTick(ctx, true)
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.curatorTick(ctx, false)
			}
		}
	})
}

func (r *Runtime) curatorTick(ctx context.Context, startup bool) {
	pass, err := r.Curator.Pass(ctx)
	if err != nil {
		logging.L().Warn("skills.curator.pass_failed", zap.Error(err))
		return
	}
	running, err := r.Curator.RunningTurns(ctx)
	if err != nil {
		logging.L().Warn("skills.curator.runs_failed", zap.Error(err))
		return
	}
	now := time.Now()
	run, first, noteIdle := skills.CuratorPassDue(now, unixMilliTime(pass.LastPassAt), unixMilliTime(pass.RunsIdleSince), running, startup)
	if first {
		pass.LastPassAt = now.UnixMilli()
		_ = r.Curator.SavePass(ctx, pass)
		return
	}
	if noteIdle {
		pass.RunsIdleSince = now.UnixMilli()
		_ = r.Curator.SavePass(ctx, pass)
		return
	}
	if run {
		r.runCuratorTransitions(ctx, &pass, now)
	}
	r.maybeCuratorMerge(ctx, &pass, now, running, startup)
}

func (r *Runtime) runCuratorTransitions(ctx context.Context, pass *db.SkillCuratorPass, now time.Time) {
	if err := r.Curator.Sync(ctx); err != nil {
		logging.L().Warn("skills.curator.sync_failed", zap.Error(err))
		return
	}
	rows, err := r.Curator.Rows(ctx)
	if err != nil {
		logging.L().Warn("skills.curator.rows_failed", zap.Error(err))
		return
	}
	archived, deleted := r.Curator.Apply(ctx, skills.PlanCuratorTransitions(now, rows))
	if len(archived) > 0 && r.Inbox != nil {
		body := "These self-improvement skills will be deleted in 10 days unless you restore them:\n" + strings.Join(archived, "\n")
		if _, err := r.Inbox.Create(ctx, inbox.CreateInput{Title: "Skills archived", Body: body}); err != nil {
			logging.L().Warn("skills.curator.inbox_failed", zap.Error(err))
		}
	}
	if len(deleted) > 0 && r.Events != nil {
		r.Events.Publish(event.SkillCuratorDeleted(len(deleted)))
	}
	pass.LastPassAt = now.UnixMilli()
	if err := r.Curator.SavePass(ctx, *pass); err != nil {
		logging.L().Warn("skills.curator.save_pass_failed", zap.Error(err))
	}
}

func (r *Runtime) maybeCuratorMerge(ctx context.Context, pass *db.SkillCuratorPass, now time.Time, running, startup bool) {
	if running || pass.LastPassAt == 0 || pass.LastMergeAt >= pass.LastPassAt {
		return
	}
	if !startup {
		idle := unixMilliTime(pass.RunsIdleSince)
		if idle.IsZero() || now.Sub(idle) < skills.CuratorIdleGate {
			return
		}
	}
	if err := r.runCuratorMerge(ctx); err != nil {
		logging.L().Warn("skills.curator.merge_failed", zap.Error(err))
		return
	}
	pass.LastMergeAt = now.UnixMilli()
	_ = r.Curator.SavePass(ctx, *pass)
}

func (r *Runtime) runCuratorMerge(ctx context.Context) error {
	providerID, modelID := r.Config.ExtractionLLM()
	if providerID == "" || modelID == "" {
		logging.L().Warn("skills.curator.merge_skipped_no_model")
		return nil
	}
	rows, err := r.Curator.Rows(ctx)
	if err != nil {
		return err
	}
	var live []skills.CuratorSkill
	pinned := map[string]bool{}
	for _, row := range rows {
		if row.Pinned {
			pinned[row.Name] = true
		}
		if row.Status == skills.CuratorStatusArchived {
			continue
		}
		live = append(live, row)
	}
	if len(live) < 2 {
		return nil
	}
	workspaces, err := r.Sessions.ListWorkspaces(ctx)
	if err != nil {
		return err
	}
	if len(workspaces) == 0 {
		return nil
	}
	parent, err := r.Sessions.NewInboxSession(ctx, workspaces[0].ID, modelID, providerID)
	if err != nil {
		return err
	}
	defer func() { _ = r.Sessions.DeleteSession(ctx, parent.ID) }()
	child, err := r.Sessions.NewChildSession(ctx, parent, "skill curator merge", skillCuratorKind)
	if err != nil {
		return err
	}
	defer func() { _ = r.Sessions.DeleteSession(ctx, child.ID) }()
	child, err = r.Sessions.UpdateSessionModel(ctx, child.ID, modelID, providerID)
	if err != nil {
		return err
	}
	prompt := curatorMergePrompt(live)
	if _, err := r.Sessions.AppendUserMessage(ctx, child.ID, prompt); err != nil {
		return err
	}
	registry := tools.NewCuratorMergeRegistry(workspaces[0].Path, nil)
	if err := r.runHiddenChild(ctx, child, registry, 8, curatorMergeSystemPrompt()); err != nil {
		return err
	}
	target, absorbed, wrote := readMergeReport(ctx, r.Sessions, child.ID)
	sources := skillcurator.MergeSources(target, absorbed, wrote, pinned)
	for _, name := range sources {
		if err := skills.ArchiveManagedSkill(name); err != nil {
			logging.L().Warn("skills.curator.merge_archive_failed", zap.String("skill", name), zap.Error(err))
			continue
		}
		if err := r.Curator.MarkArchived(ctx, name); err != nil {
			logging.L().Warn("skills.curator.merge_state_failed", zap.String("skill", name), zap.Error(err))
		}
	}
	if wrote && target != "" && len(sources) > 0 && r.Events != nil {
		r.Events.Publish(event.SkillCuratorMerged(target, sources))
	}
	return nil
}

func curatorMergeSystemPrompt() string {
	return "Merge overlapping self-improvement skills. Overwrite one surviving skill with write_skill. Then call report_merged_skills with the surviving name and the sources you fully absorbed. Do not delete files, do not edit skills that are not self-improvement, and do not create an unrelated skill."
}

func curatorMergePrompt(live []skills.CuratorSkill) string {
	var b strings.Builder
	b.WriteString("Self-improvement skills:\n")
	for _, skill := range live {
		desc := skill.Name
		if loaded, err := skills.ReadSkill(skillPath(skill.Name)); err == nil {
			desc = loaded.Description
		}
		b.WriteString("- " + skill.Name + " (" + skill.Status + "): " + desc + "\n")
	}
	return b.String()
}

func skillPath(name string) string {
	root, err := skills.MirrorRoot()
	if err != nil {
		return name
	}
	return root + "/" + name
}

func readMergeReport(ctx context.Context, sessions *session.Service, childID string) (target string, absorbed []string, wrote bool) {
	calls, err := sessions.ListToolCallsForSession(ctx, childID)
	if err != nil {
		return "", nil, false
	}
	var written string
	for _, call := range calls {
		switch call.ToolName {
		case "report_merged_skills":
			var in struct {
				Target   string   `json:"target"`
				Absorbed []string `json:"absorbed"`
			}
			if err := json.Unmarshal([]byte(call.Arguments), &in); err == nil {
				target = strings.TrimSpace(in.Target)
				absorbed = in.Absorbed
			}
		case "write_skill":
			var in struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal([]byte(call.Arguments), &in); err == nil {
				written = strings.TrimSpace(in.Name)
			}
		}
	}
	return target, absorbed, written != "" && written == target
}

func unixMilliTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}
