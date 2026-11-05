package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/paths"
	"go.uber.org/zap"
)

type handlerBinding struct {
	Type       string `json:"type"`
	Command    string `json:"command"`
	TimeoutSec int    `json:"timeout_sec"`
}

type eventRule struct {
	Matcher string           `json:"matcher"`
	Hooks   []handlerBinding `json:"hooks"`
}

type fileConfig struct {
	Hooks map[string][]eventRule `json:"hooks"`
}

type pluginManifest struct {
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Hooks map[string][]eventRule `json:"hooks"`
}

type resolvedHandler struct {
	Event      string
	Matcher    string
	Command    string
	TimeoutSec int
	Source     string
	WorkDir    string
}

func loadFileConfig(path string) (fileConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fileConfig{}, fmt.Errorf("read hooks config %s: %w", path, err)
	}
	var cfg fileConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fileConfig{}, fmt.Errorf("parse hooks config %s: %w", path, err)
	}
	if cfg.Hooks == nil {
		cfg.Hooks = map[string][]eventRule{}
	}
	return cfg, nil
}

func loadPluginManifest(path string) (pluginManifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return pluginManifest{}, fmt.Errorf("read plugin manifest %s: %w", path, err)
	}
	var m pluginManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return pluginManifest{}, fmt.Errorf("parse plugin manifest %s: %w", path, err)
	}
	if m.Hooks == nil {
		m.Hooks = map[string][]eventRule{}
	}
	return m, nil
}

func appendRules(out *[]resolvedHandler, rules []eventRule, event, source, pluginDir string) {
	for _, rule := range rules {
		for _, h := range rule.Hooks {
			if strings.TrimSpace(h.Command) == "" {
				continue
			}
			if h.Type != "" && h.Type != "command" {
				continue
			}
			*out = append(*out, resolvedHandler{
				Event:      event,
				Matcher:    rule.Matcher,
				Command:    h.Command,
				TimeoutSec: h.TimeoutSec,
				Source:     source,
				WorkDir:    pluginDir,
			})
		}
	}
}

func appendFileHooks(handlers *[]resolvedHandler, path string, workDir string) {
	cfg, err := loadFileConfig(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logging.L().Warn("hooks.config_load_failed", zap.String("path", path), zap.Error(err))
		}
		return
	}
	for event, rules := range cfg.Hooks {
		appendRules(handlers, rules, event, path, workDir)
	}
}

func appendPluginDir(handlers *[]resolvedHandler, pluginsRoot string) {
	entries, err := os.ReadDir(pluginsRoot)
	if err != nil {
		return
	}
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		pluginDir := filepath.Join(pluginsRoot, ent.Name())
		manifestPath := filepath.Join(pluginDir, "plugin.json")
		m, err := loadPluginManifest(manifestPath)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				logging.L().Warn("hooks.plugin_manifest_load_failed", zap.String("path", manifestPath), zap.Error(err))
			}
			continue
		}
		source := manifestPath
		if strings.TrimSpace(m.Name) != "" {
			source = m.Name + " (" + manifestPath + ")"
		}
		for event, rules := range m.Hooks {
			appendRules(handlers, rules, event, source, pluginDir)
		}
	}
}

func discoverHandlers(workspacePath string, projectOK bool) []resolvedHandler {
	var handlers []resolvedHandler

	dataDir, err := paths.DataDir()
	if err == nil {
		appendFileHooks(&handlers, filepath.Join(dataDir, "hooks.json"), "")
		appendPluginDir(&handlers, filepath.Join(dataDir, "plugins"))
	}

	if projectOK {
		ws := strings.TrimSpace(workspacePath)
		if ws != "" {
			appendFileHooks(&handlers, filepath.Join(ws, ".cometmind", "hooks.json"), filepath.Join(ws, ".cometmind"))
			appendPluginDir(&handlers, filepath.Join(ws, ".cometmind", "plugins"))
		}
	}

	return handlers
}

func resolveCommand(cmd, pluginDir string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}
	if pluginDir == "" {
		return cmd
	}
	if strings.Contains(cmd, "/") || strings.Contains(cmd, string(filepath.Separator)) {
		return filepath.Join(pluginDir, cmd)
	}
	local := filepath.Join(pluginDir, cmd)
	if st, err := os.Stat(local); err == nil && !st.IsDir() {
		return local
	}
	return cmd
}
