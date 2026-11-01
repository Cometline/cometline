package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Cometline/cometline/cometmind/internal/mcp/catalog"
)

// rewriteSettingsMCPServers persists catalog-synced MCP rows by patching only
// cometmind.mcp in cometline-settings.json. All other keys (including unmodeled
// fields such as cometmind.logLevel) are preserved verbatim.
func rewriteSettingsMCPServers(path string, synced MCPConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	existing := existingMCPSubtree(doc)
	next, changed := mergeSyncedMCPIntoExisting(existing, synced)
	if !changed {
		return nil
	}
	cm, ok := doc["cometmind"].(map[string]any)
	if !ok {
		cm = map[string]any{}
		doc["cometmind"] = cm
	}
	cm["mcp"] = next
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return writeSettingsFileAtomic(path, out)
}

func mergeSyncedMCPIntoExisting(existing map[string]any, synced MCPConfig) (map[string]any, bool) {
	var out map[string]any
	if existing != nil {
		out = deepCloneJSONMap(existing)
	} else {
		out = map[string]any{"enabled": synced.Enabled, "servers": []any{}}
	}
	changed := false
	if boolValue(out["enabled"]) != synced.Enabled {
		out["enabled"] = synced.Enabled
		changed = true
	}
	syncedByID := map[string]MCPServerConfig{}
	for _, s := range synced.Servers {
		syncedByID[s.ID] = s
	}
	arr, ok := out["servers"].([]any)
	if !ok {
		arr = []any{}
	}
	for i, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		s, ok := syncedByID[id]
		if !ok {
			continue
		}
		if patchCatalogSyncedServer(m, s) {
			changed = true
			arr[i] = m
		}
	}
	out["servers"] = arr
	return out, changed
}

func patchCatalogSyncedServer(m map[string]any, s MCPServerConfig) bool {
	if s.Source != MCPServerSourceCatalog {
		return false
	}
	changed := false
	if setIfJSONDiff(m, "name", s.Name) {
		changed = true
	}
	if setIfJSONDiff(m, "transport", string(s.Transport)) {
		changed = true
	}
	if setIfJSONDiff(m, "url", s.URL) {
		changed = true
	}
	if setIfJSONDiff(m, "catalogVersion", s.CatalogVersion) {
		changed = true
	}
	if setIfJSONDiff(m, "allowedTools", s.AllowedTools) {
		changed = true
	}
	var scopes []string
	if s.OAuth != nil {
		scopes = append([]string(nil), s.OAuth.Scopes...)
	}
	if patchOAuthScopes(m, scopes) {
		changed = true
	}
	return changed
}

func patchOAuthScopes(m map[string]any, scopes []string) bool {
	if len(scopes) == 0 {
		oauth, ok := m["oauth"].(map[string]any)
		if !ok || oauth == nil {
			return false
		}
		if _, has := oauth["scopes"]; !has {
			return false
		}
		delete(oauth, "scopes")
		if len(oauth) == 0 {
			m["oauth"] = nil
		}
		return true
	}
	oauth, ok := m["oauth"].(map[string]any)
	if !ok || oauth == nil {
		oauth = map[string]any{}
		m["oauth"] = oauth
	}
	return setIfJSONDiff(oauth, "scopes", scopes)
}

func setIfJSONDiff(m map[string]any, key string, val any) bool {
	if jsonValuesEqual(m[key], val) {
		return false
	}
	m[key] = val
	return true
}

func jsonValuesEqual(a, b any) bool {
	ab, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bb, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return bytes.Equal(ab, bb)
}

func boolValue(v any) bool {
	b, _ := v.(bool)
	return b
}

func existingMCPSubtree(doc map[string]any) map[string]any {
	cm, _ := doc["cometmind"].(map[string]any)
	if cm == nil {
		return nil
	}
	mcp, _ := cm["mcp"].(map[string]any)
	return mcp
}

func deepCloneJSONMap(m map[string]any) map[string]any {
	raw, err := json.Marshal(m)
	if err != nil {
		return m
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return m
	}
	return out
}

func writeSettingsFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer tmp.Close()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename settings: %w", err)
	}
	return nil
}

func mcpConfigToJSON(cfg MCPConfig) cometlineMCPJSON {
	out := cometlineMCPJSON{Enabled: cfg.Enabled}
	for _, srv := range cfg.Servers {
		row := cometlineMCPServerJSON{
			ID:             srv.ID,
			Name:           srv.Name,
			Enabled:        srv.Enabled,
			Transport:      string(srv.Transport),
			Command:        srv.Command,
			Args:           append([]string(nil), srv.Args...),
			Env:            copyStringMapGo(srv.Env),
			URL:            srv.URL,
			Headers:        copyStringMapGo(srv.Headers),
			AllowedTools:   append([]string(nil), srv.AllowedTools...),
			Source:         string(srv.Source),
			CatalogID:      srv.CatalogID,
			CatalogVersion: srv.CatalogVersion,
		}
		if srv.OAuth != nil {
			row.OAuth = &cometlineMCPOAuthJSON{
				ClientID:         srv.OAuth.ClientID,
				Scopes:           append([]string(nil), srv.OAuth.Scopes...),
				AuthorizationURL: srv.OAuth.AuthorizationURL,
				TokenURL:         srv.OAuth.TokenURL,
			}
		}
		out.Servers = append(out.Servers, row)
	}
	return out
}

// catalogHostKey mirrors frontend dedupe (NormalizeServerURL + host/path).
func catalogHostKey(rawURL string) string {
	return catalog.HostKey(rawURL)
}
