package retention

import (
	"os"
	"path/filepath"
	"time"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/paths"
)

// PurgeRuntimeFiles deletes aged files under tool-output/ and agent-tmp/.
// It never removes the directories themselves. Errors are logged and skipped.
func PurgeRuntimeFiles(cfg config.StorageConfig) (toolOutputDeleted, agentTmpDeleted int) {
	if cfg.ToolOutputRetentionDays > 0 {
		dir, err := paths.ToolOutputDir()
		if err != nil {
			logging.L().Warn("retention.tool_output.dir_failed", "error", err)
		} else {
			toolOutputDeleted = purgeDirByAge(dir, time.Duration(cfg.ToolOutputRetentionDays)*24*time.Hour)
		}
	}
	if cfg.AgentTmpRetentionDays > 0 {
		dir, err := paths.AgentTmpDir()
		if err != nil {
			logging.L().Warn("retention.agent_tmp.dir_failed", "error", err)
		} else {
			agentTmpDeleted = purgeDirByAge(dir, time.Duration(cfg.AgentTmpRetentionDays)*24*time.Hour)
		}
	}
	return toolOutputDeleted, agentTmpDeleted
}

func purgeDirByAge(dir string, maxAge time.Duration) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			logging.L().Warn("retention.runtime_files.read_failed", "dir", dir, "error", err)
		}
		return 0
	}
	cutoff := time.Now().Add(-maxAge)
	deleted := 0
	for _, ent := range entries {
		info, err := ent.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		path := filepath.Join(dir, ent.Name())
		if err := os.RemoveAll(path); err != nil {
			logging.L().Warn("retention.runtime_files.remove_failed", "path", path, "error", err)
			continue
		}
		deleted++
	}
	return deleted
}
