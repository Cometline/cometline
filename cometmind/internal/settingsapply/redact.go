package settingsapply

import (
	"encoding/json"
	"strings"
)

// RedactSecrets walks a settings document and replaces known secret string
// fields with SecretSentinel while recording has_value alongside each secret
// as "<field>_has_value".
func RedactSecrets(doc map[string]any) map[string]any {
	out := deepCloneMap(doc)
	redactProviders(out)
	if cm, ok := asMap(out["cometmind"]); ok {
		redactMemoryEmbedding(cm)
		redactDiscord(cm)
		out["cometmind"] = cm
	}
	return out
}

func redactProviders(doc map[string]any) {
	arr, ok := doc["providers"].([]any)
	if !ok {
		return
	}
	for i, item := range arr {
		m, ok := asMap(item)
		if !ok {
			continue
		}
		redactStringField(m, "apiKey")
		arr[i] = m
	}
	doc["providers"] = arr
}

func redactMemoryEmbedding(cm map[string]any) {
	mem, ok := asMap(cm["memory"])
	if !ok {
		return
	}
	emb, ok := asMap(mem["embedding"])
	if !ok {
		return
	}
	redactStringField(emb, "apiKey")
	mem["embedding"] = emb
	cm["memory"] = mem
}

func redactDiscord(cm map[string]any) {
	gw, ok := asMap(cm["gateway"])
	if !ok {
		return
	}
	discord, ok := asMap(gw["discord"])
	if !ok {
		return
	}
	redactStringField(discord, "botToken")
	gw["discord"] = discord
	cm["gateway"] = gw
}

func redactStringField(m map[string]any, key string) {
	raw, exists := m[key]
	has := false
	if exists {
		if s, ok := raw.(string); ok {
			has = strings.TrimSpace(s) != ""
		} else if raw != nil {
			has = true
		}
	}
	m[key] = SecretSentinel
	m[key+"_has_value"] = has
}

func deepCloneMap(m map[string]any) map[string]any {
	b, err := json.Marshal(m)
	if err != nil {
		return cloneMap(m)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return cloneMap(m)
	}
	return out
}

// MergePatch deep-merges patch into base. Secret sentinel strings keep the
// previous value. *_has_value keys in the patch are ignored (read-only metadata).
func MergePatch(base, patch map[string]any) map[string]any {
	out := deepCloneMap(base)
	mergeInto(out, patch)
	stripHasValueKeys(out)
	return out
}

func mergeInto(dst, patch map[string]any) {
	for k, pv := range patch {
		if stringsHasSuffix(k, "_has_value") {
			continue
		}
		if IsSecretSentinel(pv) {
			continue
		}
		dv, exists := dst[k]
		pm, pIsMap := asMap(pv)
		dm, dIsMap := asMap(dv)
		if pIsMap && dIsMap {
			mergeInto(dm, pm)
			dst[k] = dm
			continue
		}
		if pIsMap && !exists {
			dst[k] = deepCloneMap(pm)
			continue
		}
		// Arrays and scalars: replace wholesale (including provider list).
		dst[k] = deepCloneValue(pv)
	}
}

// RestoreSecretsInProviders merges provider apiKeys by id when patch used sentinels.
// Call after MergePatch when providers were replaced as an array of objects that
// still contain sentinels (MergePatch already skipped sentinel leafs inside maps,
// but a full providers array replace may carry sentinel apiKeys that need restore).
func RestoreSecretsInProviders(merged, before map[string]any) {
	beforeByID := providerAPIKeysByID(before)
	arr, ok := merged["providers"].([]any)
	if !ok {
		return
	}
	for i, item := range arr {
		m, ok := asMap(item)
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		if IsSecretSentinel(m["apiKey"]) {
			if prev, ok := beforeByID[id]; ok {
				m["apiKey"] = prev
			} else {
				m["apiKey"] = ""
			}
		}
		delete(m, "apiKey_has_value")
		arr[i] = m
	}
	merged["providers"] = arr

	if cm, ok := asMap(merged["cometmind"]); ok {
		if mem, ok := asMap(cm["memory"]); ok {
			if emb, ok := asMap(mem["embedding"]); ok {
				if IsSecretSentinel(emb["apiKey"]) {
					if bcm, ok := asMap(before["cometmind"]); ok {
						if bmem, ok := asMap(bcm["memory"]); ok {
							if bemb, ok := asMap(bmem["embedding"]); ok {
								emb["apiKey"] = bemb["apiKey"]
							}
						}
					}
				}
				delete(emb, "apiKey_has_value")
				mem["embedding"] = emb
			}
			cm["memory"] = mem
		}
		if gw, ok := asMap(cm["gateway"]); ok {
			if discord, ok := asMap(gw["discord"]); ok {
				if IsSecretSentinel(discord["botToken"]) {
					if bcm, ok := asMap(before["cometmind"]); ok {
						if bgw, ok := asMap(bcm["gateway"]); ok {
							if bdiscord, ok := asMap(bgw["discord"]); ok {
								discord["botToken"] = bdiscord["botToken"]
							}
						}
					}
				}
				delete(discord, "botToken_has_value")
				gw["discord"] = discord
			}
			cm["gateway"] = gw
		}
		merged["cometmind"] = cm
	}
}

func providerAPIKeysByID(doc map[string]any) map[string]any {
	out := map[string]any{}
	arr, ok := doc["providers"].([]any)
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
		out[id] = m["apiKey"]
	}
	return out
}

func stripHasValueKeys(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			if stringsHasSuffix(k, "_has_value") {
				delete(t, k)
				continue
			}
			stripHasValueKeys(child)
		}
	case []any:
		for _, child := range t {
			stripHasValueKeys(child)
		}
	}
}

func stringsHasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func deepCloneValue(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return v
	}
	return out
}
