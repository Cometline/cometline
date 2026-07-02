package scheduler

const (
	CreatedByUser  = "user"
	CreatedByAgent = "agent"

	PlatformDesktop = "desktop"
	PlatformDiscord = "discord"
)

// ScheduledJob is a deferred job definition. The first cut supports one-shot
// run_at schedules; cron_expr is reserved for recurring schedules.
type ScheduledJob struct {
	ID               string
	Description      string
	DefinitionOfDone string
	WorkspacePath    string
	CreatedBy        string
	SourceSessionID  string
	SourcePlatform   string
	SourceChannelID  string
	CronExpr         string
	RunAt            *int64
	NextRunAt        int64
	LastRunAt        *int64
	Enabled          bool
	CreatedAt        int64
	UpdatedAt        int64
}

type CreateInput struct {
	Description      string
	DefinitionOfDone string
	WorkspacePath    string
	CreatedBy        string
	SourceSessionID  string
	SourcePlatform   string
	SourceChannelID  string
	CronExpr         string
	RunAt            int64
}

type UpdateInput struct {
	Description      string
	DefinitionOfDone string
	WorkspacePath    string
	RunAt            int64
	Enabled          bool
}
