package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/logging"
)

const (
	taskOutcomeRetentionDays    = 30
	taskOutcomeLatestPerLineage = 5
	taskRollupBatch             = 128
	taskSummaryMaxItems         = 12
	taskSummaryMaxFragments     = 6
	taskSummaryMaxFragmentRunes = 240
	taskSummaryMaxRunes         = 800
)

type TaskSummary struct {
	Summary         string   `json:"summary"`
	Status          string   `json:"status"`
	Decisions       []string `json:"decisions"`
	Artifacts       []string `json:"artifacts"`
	Failures        []string `json:"failures"`
	OpenItems       []string `json:"open_items"`
	LastCompletedAt int64    `json:"last_completed_at"`
}

type TaskOutcomeInput struct {
	OriginType      string
	OriginID        string
	Status          string
	Description     string
	Progress        string
	Decisions       []string
	Artifacts       []string
	Failures        []string
	OpenItems       []string
	LastCompletedAt time.Time
}

func (s *Service) RecordTaskOutcome(ctx context.Context, in TaskOutcomeInput) (Record, error) {
	if in.OriginType != "job" && in.OriginType != "scheduled_job" {
		return Record{}, fmt.Errorf("task outcome origin_type must be job or scheduled_job")
	}
	if strings.TrimSpace(in.OriginID) == "" {
		return Record{}, fmt.Errorf("task outcome origin_id is required")
	}
	if in.LastCompletedAt.IsZero() {
		in.LastCompletedAt = time.Now()
	}
	payload := TaskSummary{
		Summary: taskOutcomeSummary(in), Status: strings.TrimSpace(in.Status), Decisions: compactStrings(in.Decisions),
		Artifacts: compactStrings(in.Artifacts), Failures: compactStrings(in.Failures),
		OpenItems: compactStrings(in.OpenItems), LastCompletedAt: in.LastCompletedAt.UnixMilli(),
	}
	normalizeTaskSummary(&payload)
	raw, err := json.Marshal(payload)
	if err != nil {
		return Record{}, err
	}
	content := conciseOutcomeContent(in)
	vecs, err := s.retriever.embedder.Embed(ctx, content)
	if err != nil {
		return Record{}, err
	}
	if len(vecs) == 0 {
		return Record{}, fmt.Errorf("embedding failed")
	}
	rec := Record{
		ID: NewID(), Scope: "global", Kind: "task_outcome", Content: content,
		Embedding: vecs[0], EmbeddingModel: s.retriever.embedder.Model(), Source: in.OriginType,
		BaseWeight: 1, ApplicationPolicy: ApplicationRelevant, RetentionPolicy: RetentionDecaying,
		OriginType: in.OriginType, OriginID: in.OriginID, SummaryJSON: string(raw),
		LastAccessedAt: &in.LastCompletedAt, CreatedAt: in.LastCompletedAt, UpdatedAt: in.LastCompletedAt,
	}
	s.outcomeMu.Lock()
	defer s.outcomeMu.Unlock()
	if err := s.store.insert(ctx, rec); err != nil {
		return Record{}, err
	}
	rollUp := s.rollUpTaskLineage
	if s.rollUpTaskLineageOverride != nil {
		rollUp = s.rollUpTaskLineageOverride
	}
	if err := rollUp(ctx, in.OriginType, in.OriginID); err != nil {
		logging.L().Warn("memory.task_outcome.rollup_failed", "memory_id", rec.ID, "origin_type", in.OriginType, "origin_id", in.OriginID, "error", err)
	}
	return rec, nil
}

func (s *Service) rollUpTaskLineage(ctx context.Context, originType, originID string) error {
	rows, err := s.store.q.ListTaskMemoriesByLineage(ctx, db.ListTaskMemoriesByLineageParams{
		OriginType: originType, OriginID: originID, Limit: taskRollupBatch,
	})
	if err != nil {
		return err
	}
	var priorSummary *Record
	outcomes := make([]Record, 0, len(rows))
	for _, row := range rows {
		rec := recordFromDB(row)
		if rec.Kind == "task_summary" {
			if priorSummary == nil {
				copy := rec
				priorSummary = &copy
			}
			continue
		}
		outcomes = append(outcomes, rec)
	}
	sort.Slice(outcomes, func(i, j int) bool { return outcomes[i].CreatedAt.After(outcomes[j].CreatedAt) })
	cutoff := time.Now().Add(-taskOutcomeRetentionDays * 24 * time.Hour)
	consumed := make([]Record, 0)
	for i, rec := range outcomes {
		if i < taskOutcomeLatestPerLineage || !rec.CreatedAt.Before(cutoff) {
			continue
		}
		consumed = append(consumed, rec)
	}
	if len(consumed) == 0 {
		return nil
	}

	aggregate := TaskSummary{}
	if priorSummary != nil {
		_ = json.Unmarshal([]byte(priorSummary.SummaryJSON), &aggregate)
	}
	sort.Slice(consumed, func(i, j int) bool { return consumed[i].CreatedAt.Before(consumed[j].CreatedAt) })
	summaries := []string{aggregate.Summary}
	for _, rec := range consumed {
		var item TaskSummary
		if json.Unmarshal([]byte(rec.SummaryJSON), &item) != nil {
			item.Status = strings.TrimSpace(rec.Content)
		}
		if strings.TrimSpace(item.Summary) == "" {
			item.Summary = strings.TrimSpace(rec.Content)
		}
		summaries = append(summaries, item.Summary)
		aggregate.Status = item.Status
		aggregate.Decisions = appendUnique(aggregate.Decisions, item.Decisions...)
		aggregate.Artifacts = appendUnique(aggregate.Artifacts, item.Artifacts...)
		aggregate.Failures = appendUnique(aggregate.Failures, item.Failures...)
		aggregate.OpenItems = appendUnique(aggregate.OpenItems, item.OpenItems...)
		if item.LastCompletedAt > aggregate.LastCompletedAt {
			aggregate.LastCompletedAt = item.LastCompletedAt
		}
	}
	aggregate.Summary = mergeTaskSummaryText(summaries...)
	normalizeTaskSummary(&aggregate)
	raw, err := json.Marshal(aggregate)
	if err != nil {
		return err
	}
	content := "Task history: " + aggregate.Summary
	if aggregate.Status != "" {
		content += "\nLatest rolled-up status: " + aggregate.Status
	}
	vecs, err := s.retriever.embedder.Embed(ctx, content)
	if err != nil || len(vecs) == 0 {
		if err != nil {
			return err
		}
		return fmt.Errorf("embedding task summary failed")
	}
	now := time.Now()
	summary := Record{
		ID: NewID(), Scope: "global", Kind: "task_summary", Content: content,
		Embedding: vecs[0], EmbeddingModel: s.retriever.embedder.Model(), Source: "rollup",
		BaseWeight: 1, ApplicationPolicy: ApplicationRelevant, RetentionPolicy: RetentionProtected,
		OriginType: originType, OriginID: originID, SummaryJSON: string(raw),
		LastAccessedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	return s.replaceTaskSummary(ctx, priorSummary, consumed, summary)
}

func (s *Service) replaceTaskSummary(ctx context.Context, prior *Record, consumed []Record, summary Record) error {
	tx, err := s.store.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := db.New(tx)
	now := time.Now().UnixMilli()
	archive := func(id string) error {
		if err := q.ArchiveMemory(ctx, db.ArchiveMemoryParams{ArchivedReason: nullString("task_rollup"), SupersededBy: nullString(summary.ID), UpdatedAt: now, ID: id}); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM memories_fts WHERE memory_id = ?`, id)
		return err
	}
	if prior != nil {
		if err := archive(prior.ID); err != nil {
			return err
		}
	}
	for _, rec := range consumed {
		if err := archive(rec.ID); err != nil {
			return err
		}
	}
	if err := q.InsertMemory(ctx, db.InsertMemoryParams{
		ID: summary.ID, Scope: summary.Scope, Kind: summary.Kind, PreferenceCategory: "",
		Content: summary.Content, Embedding: encodeEmbedding(summary.Embedding), EmbeddingModel: nullString(summary.EmbeddingModel),
		Source: summary.Source, BaseWeight: summary.BaseWeight, AccessCount: 0,
		ApplicationPolicy: summary.ApplicationPolicy, RetentionPolicy: summary.RetentionPolicy,
		OriginType: summary.OriginType, OriginID: summary.OriginID, SummaryJson: summary.SummaryJSON,
		SourceSessionID: sql.NullString{}, SupersededBy: sql.NullString{}, Archived: 0,
		ArchivedReason: sql.NullString{}, LastAccessedAt: nullInt64MS(summary.LastAccessedAt), CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO memories_fts (memory_id, content) VALUES (?, ?)`, summary.ID, summary.Content); err != nil {
		return err
	}
	return tx.Commit()
}

func conciseOutcomeContent(in TaskOutcomeInput) string {
	status := strings.TrimSpace(in.Status)
	summary := taskOutcomeSummary(in)
	if status == "" {
		return summary
	}
	return fmt.Sprintf("Task %s: %s", status, summary)
}

func compactStrings(values []string) []string {
	return appendUnique(make([]string, 0, min(len(values), taskSummaryMaxItems)), values...)
}

func normalizeTaskSummary(summary *TaskSummary) {
	if summary.Decisions == nil {
		summary.Decisions = []string{}
	}
	if summary.Artifacts == nil {
		summary.Artifacts = []string{}
	}
	if summary.Failures == nil {
		summary.Failures = []string{}
	}
	if summary.OpenItems == nil {
		summary.OpenItems = []string{}
	}
}

func appendUnique(dst []string, values ...string) []string {
	seen := make(map[string]struct{}, len(dst)+len(values))
	for _, value := range dst {
		seen[value] = struct{}{}
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		dst = append(dst, value)
		if len(dst) > taskSummaryMaxItems {
			dst = dst[len(dst)-taskSummaryMaxItems:]
		}
	}
	return dst
}

func taskOutcomeSummary(in TaskOutcomeInput) string {
	description := boundedRunes(strings.TrimSpace(in.Description), taskSummaryMaxFragmentRunes)
	progress := boundedRunes(strings.TrimSpace(in.Progress), taskSummaryMaxFragmentRunes)
	switch {
	case description == "":
		return progress
	case progress == "" || progress == description:
		return description
	default:
		return boundedRunes(description+"; "+progress, taskSummaryMaxRunes)
	}
}

func mergeTaskSummaryText(values ...string) string {
	fragments := make([]string, 0, taskSummaryMaxFragments)
	seen := map[string]struct{}{}
	for _, value := range values {
		for _, fragment := range strings.Split(value, " | ") {
			fragment = boundedRunes(strings.TrimSpace(fragment), taskSummaryMaxFragmentRunes)
			if fragment == "" {
				continue
			}
			if _, ok := seen[fragment]; ok {
				continue
			}
			seen[fragment] = struct{}{}
			fragments = append(fragments, fragment)
			if len(fragments) > taskSummaryMaxFragments {
				delete(seen, fragments[0])
				fragments = fragments[1:]
			}
		}
	}
	for len(fragments) > 1 && len([]rune(strings.Join(fragments, " | "))) > taskSummaryMaxRunes {
		fragments = fragments[1:]
	}
	return boundedRunes(strings.Join(fragments, " | "), taskSummaryMaxRunes)
}

func boundedRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return strings.TrimSpace(string(runes[:limit]))
}
