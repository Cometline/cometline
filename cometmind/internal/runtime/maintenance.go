package runtime

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/backup"
	"github.com/Cometline/cometline/cometmind/internal/config"
	"github.com/Cometline/cometline/cometmind/internal/inbox"
	"github.com/Cometline/cometline/cometmind/internal/jobs"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/memory"
	"github.com/Cometline/cometline/cometmind/internal/retention"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/usage"
)

// StartScheduler materializes due scheduled jobs into the normal jobs queue.
func (r *Runtime) StartScheduler(ctx context.Context) {
	if r == nil || r.Scheduler == nil || r.Jobs == nil {
		return
	}
	cfg := r.Config.EffectiveSchedulerSettings()
	if !cfg.Enabled {
		return
	}
	interval := time.Duration(cfg.PollIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				count, err := r.Scheduler.MaterializeDue(ctx, r.Jobs, 0)
				if err != nil {
					logging.L().Warn("scheduler.materialize.failed", "error", err)
				} else if count > 0 {
					logging.L().Info("scheduler.materialized", "count", count)
				}
			}
		}
	}()
}

// StartJobsMaintenance runs periodic orphan reconcile and job lifecycle maintenance.
// The reconcile interval is re-read each cycle so Reload can change it live.
func (r *Runtime) StartJobsMaintenance(ctx context.Context) {
	if r == nil || r.Jobs == nil {
		return
	}
	go func() {
		for {
			if !waitForMaintenance(ctx, r.jobsChanged, func() (bool, time.Duration) {
				interval := time.Duration(r.jobSettingsSnapshot().ReconcileIntervalS) * time.Second
				if interval <= 0 {
					interval = jobs.DefaultReconcileInterval
				}
				return true, interval
			}) {
				return
			}
			if _, err := r.Jobs.Reconcile(ctx, r.isRunning); err != nil {
				logging.L().Warn("jobs.reconcile.failed", "error", err)
			}
			settings := r.jobSettingsSnapshot()
			if stale, err := r.Jobs.StaleOngoing(ctx, time.Duration(settings.StaleReviewMinutes)*time.Minute); err != nil {
				logging.L().Warn("jobs.stale_review.failed", "error", err)
			} else {
				for _, job := range stale {
					logging.L().Warn("jobs.stale_ongoing", "job_id", job.ID, "assigned_session_id", job.AssignedSessionID, "updated_at", job.UpdatedAt)
				}
			}
			if _, err := r.Jobs.ArchiveDone(ctx, settings.DoneArchiveDays); err != nil {
				logging.L().Warn("jobs.done_archive.failed", "error", err)
			}
			if _, err := r.Jobs.PurgeArchived(ctx, settings.ArchivedPurgeDays); err != nil {
				logging.L().Warn("jobs.archive_purge.failed", "error", err)
			}
		}
	}()
}

func waitForMaintenance(ctx context.Context, changed <-chan struct{}, snapshot func() (bool, time.Duration)) bool {
	var cycleStarted time.Time
	for {
		enabled, interval := snapshot()
		if !enabled {
			cycleStarted = time.Time{}
			select {
			case <-ctx.Done():
				return false
			case <-changed:
				continue
			}
		}
		if cycleStarted.IsZero() {
			cycleStarted = time.Now()
		}
		remaining := time.Until(cycleStarted.Add(interval))
		if remaining <= 0 {
			return true
		}
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return false
		case <-changed:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-timer.C:
			return true
		}
	}
}

// StartRetentionMaintenance runs full storage retention on the configured interval.
// Interval and enablement are re-read each cycle so Reload can change them live.
func (r *Runtime) StartRetentionMaintenance(ctx context.Context) {
	if r == nil {
		return
	}
	go func() {
		for {
			if !waitForMaintenance(ctx, r.retentionChanged, func() (bool, time.Duration) {
				cfg := r.Config.EffectiveStorageConfig()
				enabled := cfg.RetentionEnabled() || cfg.DetachedMediaRetentionEnabled() || cfg.MemoryPurgeEnabled() || r.jobSettingsSnapshot().DeletedPurgeDays > 0 || cfg.RuntimeFilesEnabled() || r.Usage != nil
				interval := time.Duration(cfg.CleanupIntervalMinutes) * time.Minute
				if interval <= 0 {
					interval = time.Hour
				}
				return enabled, interval
			}) {
				return
			}
			result, err := r.RunRetention(ctx)
			if err != nil {
				logging.L().Warn("retention.failed", "error", err)
				continue
			}
			if result.SessionsDeleted > 0 || result.SubagentsDeleted > 0 || result.MediaDeleted > 0 || result.MemoriesPurged > 0 || result.MemoryEventsPurged > 0 || result.JobsPurged > 0 || result.InboxPurged > 0 || result.UsageEventsPurged > 0 || result.ToolOutputDeleted > 0 || result.AgentTmpDeleted > 0 {
				logging.L().Info("retention.completed",
					"sessions_deleted", result.SessionsDeleted,
					"subagents_deleted", result.SubagentsDeleted,
					"media_deleted", result.MediaDeleted,
					"memories_purged", result.MemoriesPurged,
					"memory_events_purged", result.MemoryEventsPurged,
					"jobs_purged", result.JobsPurged,
					"inbox_purged", result.InboxPurged,
					"usage_events_purged", result.UsageEventsPurged,
					"tool_output_deleted", result.ToolOutputDeleted,
					"agent_tmp_deleted", result.AgentTmpDeleted,
					"vacuumed", result.Vacuumed,
				)
			}
		}
	}()
}

func runRetention(ctx context.Context, db *sql.DB, sessions *session.Service, mem *memory.Service, jobSvc *jobs.Service, inboxSvc *inbox.Service, usageSvc *usage.Service, cfg config.StorageConfig, inboxCfg config.InboxConfig, jobPurgeDays int, isRunning func(string) bool) (retention.Result, error) {
	var out retention.Result
	needDB := cfg.RetentionEnabled() || cfg.DetachedMediaRetentionEnabled() || cfg.MemoryPurgeEnabled() || jobPurgeDays > 0 || inboxSvc != nil || usageSvc != nil
	if needDB {
		rr := &retention.Runner{
			DB:           db,
			Sessions:     sessions,
			Memory:       mem,
			Jobs:         jobSvc,
			Inbox:        inboxSvc,
			Usage:        usageSvc,
			Config:       cfg,
			InboxCfg:     inboxCfg,
			JobPurgeDays: jobPurgeDays,
			IsRunning:    isRunning,
			VacuumAsync:  true,
		}
		var err error
		out, err = rr.Run(ctx)
		if err != nil {
			return out, err
		}
	}
	if cfg.RuntimeFilesEnabled() {
		to, at := retention.PurgeRuntimeFiles(cfg)
		out.ToolOutputDeleted = to
		out.AgentTmpDeleted = at
		if to > 0 || at > 0 {
			logging.L().Info("retention.runtime_files",
				"tool_output_deleted", to,
				"agent_tmp_deleted", at,
			)
		}
	}
	return out, nil
}

// RunRetention executes the configured storage retention rules once.
func (r *Runtime) RunRetention(ctx context.Context) (retention.Result, error) {
	if r == nil {
		return retention.Result{}, fmt.Errorf("runtime is nil")
	}
	r.retentionMu.Lock()
	defer r.retentionMu.Unlock()
	result, err := runRetention(ctx, r.DB, r.Sessions, r.Memory, r.Jobs, r.Inbox, r.Usage, r.Config.EffectiveStorageConfig(), r.Config.EffectiveInboxSettings(), r.jobSettingsSnapshot().DeletedPurgeDays, r.isRunning)
	if err != nil {
		return result, err
	}
	return result, nil
}

// RunBackup creates one zip archive of ~/.cometmind in the configured destination.
func (r *Runtime) RunBackup(ctx context.Context) (backup.Result, error) {
	if r == nil {
		return backup.Result{}, fmt.Errorf("runtime is nil")
	}
	cfg := r.Config.EffectiveStorageConfig().Backup
	if strings.TrimSpace(cfg.DestinationDir) == "" {
		return backup.Result{}, fmt.Errorf("backup destination directory is not configured")
	}
	return (&backup.Archiver{DB: r.DB}).Run(ctx, backup.Config{
		DestinationDir: cfg.DestinationDir,
		MaxBackups:     cfg.MaxBackups,
	})
}

// StartBackupMaintenance runs automatic zip backups on the configured interval.
func (r *Runtime) StartBackupMaintenance(ctx context.Context) {
	if r == nil {
		return
	}
	go func() {
		for {
			if !waitForMaintenance(ctx, r.backupChanged, func() (bool, time.Duration) {
				cfg := r.Config.EffectiveStorageConfig()
				interval := time.Duration(cfg.Backup.IntervalHours) * time.Hour
				if interval <= 0 {
					interval = 24 * time.Hour
				}
				return cfg.BackupEnabled(), interval
			}) {
				return
			}
			result, err := r.RunBackup(ctx)
			if err != nil {
				logging.L().Warn("backup.failed", "error", err)
				continue
			}
			logging.L().Info("backup.completed",
				"path", result.Path,
				"files_zipped", result.FilesZipped,
				"removed_old", result.RemovedOld,
			)
		}
	}()
}
