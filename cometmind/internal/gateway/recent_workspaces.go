package gateway

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/cometline/cometmind/internal/paths"
)

type cometlineWorkspaceStore struct {
	WorkspacePath string   `json:"workspacePath"`
	RecentPaths   []string `json:"recentPaths"`
}

func recentWorkspacePaths(fallback string) []string {
	var out []string
	seen := make(map[string]struct{})
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}

	storePath, err := paths.WorkspaceStorePath()
	if err == nil {
		raw, err := os.ReadFile(storePath)
		if err == nil {
			var store cometlineWorkspaceStore
			if json.Unmarshal(raw, &store) == nil {
				add(store.WorkspacePath)
				for _, path := range store.RecentPaths {
					add(path)
				}
			}
		}
	}

	add(fallback)
	return out
}
