package scheduler

const (
	CreatedByUser  = "user"
	CreatedByAgent = "agent"

	PlatformDesktop = "desktop"
	PlatformDiscord = "discord"
)

// ScheduledJob is a deferred job definition. It supports one-shot run_at
// schedules and standard cron_expr recurring schedules.
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
	CronExpr         string
	RunAt            int64
	Enabled          bool
}
