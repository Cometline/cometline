package settingsapply

import "strings"

// RestoreMCPSecretsInCometmind merges MCP env/header secrets by server id when patch used sentinels.
func RestoreMCPSecretsInCometmind(merged, before map[string]any) {
	beforeServers := mcpServersByID(before)
	cm, ok := asMap(merged["cometmind"])
	if !ok {
		return
	}
	mcp, ok := asMap(cm["mcp"])
	if !ok {
		return
	}
	arr, ok := mcp["servers"].([]any)
	if !ok {
		return
	}
	for i, item := range arr {
		m, ok := asMap(item)
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		prev := beforeServers[id]
		if prev == nil {
			continue
		}
		m["env"] = restoreStringMapSecrets(m["env"], prev["env"])
		m["headers"] = restoreStringMapSecrets(m["headers"], prev["headers"])
		stripMCPHasValueMetadata(m)
		arr[i] = m
	}
	mcp["servers"] = arr
	cm["mcp"] = mcp
	merged["cometmind"] = cm
}

func mcpServersByID(doc map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	cm, ok := asMap(doc["cometmind"])
	if !ok {
		return out
	}
	mcp, ok := asMap(cm["mcp"])
	if !ok {
		return out
	}
	arr, ok := mcp["servers"].([]any)
	if !ok {
		return out
	}
	for _, item := range arr {
		m, ok := asMap(item)
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		if id == "" {
			continue
		}
		out[id] = m
	}
	return out
}

func restoreStringMapSecrets(patchVal, beforeVal any) any {
	pm, ok := asMap(patchVal)
	if !ok {
		return patchVal
	}
	bm, _ := asMap(beforeVal)
	out := map[string]any{}
	for k, pv := range pm {
		if stringsHasSuffix(k, "_has_value") {
			continue
		}
		if IsSecretSentinel(pv) {
			if bm != nil {
				if prev, ok := bm[k]; ok {
					out[k] = prev
					continue
				}
			}
			out[k] = ""
			continue
		}
		out[k] = pv
	}
	return out
}

func redactMCP(cm map[string]any) {
	mcp, ok := asMap(cm["mcp"])
	if !ok {
		return
	}
	arr, ok := mcp["servers"].([]any)
	if !ok {
		return
	}
	for i, item := range arr {
		m, ok := asMap(item)
		if !ok {
			continue
		}
		m["env"] = redactStringMap(m["env"])
		m["headers"] = redactStringMap(m["headers"])
		arr[i] = m
	}
	mcp["servers"] = arr
	cm["mcp"] = mcp
}

func redactStringMap(raw any) any {
	m, ok := asMap(raw)
	if !ok || len(m) == 0 {
		return raw
	}
	out := map[string]any{}
	for k, v := range m {
		s, _ := v.(string)
		has := strings.TrimSpace(s) != ""
		out[k] = SecretSentinel
		out[k+"_has_value"] = has
	}
	return out
}
