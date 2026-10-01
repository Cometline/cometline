package jobs

import (
	"database/sql"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/db"
)

func jobFromRow(row db.Job) Job {
	return Job{
		ID:                row.ID,
		Description:       row.Description,
		DefinitionOfDone:  row.DefinitionOfDone,
		Progress:          row.Progress,
		Status:            row.Status,
		WorkspacePath:     nullStringVal(row.WorkspacePath),
		AssignedSessionID: nullStringVal(row.AssignedSessionID),
		LeaseExpiresAt:    nullInt64Ptr(row.LeaseExpiresAt),
		CreatedBy:         row.CreatedBy,
		SourceSessionID:   nullStringVal(row.SourceSessionID),
		SourcePlatform:    row.SourcePlatform,
		SourceChannelID:   nullStringVal(row.SourceChannelID),
		ArchivedAt:        nullInt64Ptr(row.ArchivedAt),
		FailureCount:      row.FailureCount,
		NextRetryAt:       nullInt64Ptr(row.NextRetryAt),
		LastFailureReason: nullStringVal(row.LastFailureReason),
		DeletedAt:         nullInt64Ptr(row.DeletedAt),
		ScheduledJobID:    nullStringVal(row.ScheduledJobID),
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

func eventFromRow(row db.JobEvent) JobEvent {
	return JobEvent{
		ID:             row.ID,
		JobID:          row.JobID,
		Action:         row.Action,
		Detail:         row.Detail,
		ActorSessionID: nullStringVal(row.ActorSessionID),
		CreatedAt:      row.CreatedAt,
	}
}

func nullInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}

func nullStringVal(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func optionalNullString(v string) sql.NullString {
	v = strings.TrimSpace(v)
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}
