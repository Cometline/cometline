package settingsapply

// stripMCPHasValueMetadata removes *_has_value keys mistakenly stored in MCP env/header maps.
func stripMCPHasValueMetadata(m map[string]any) {
	if m == nil {
		return
	}
	env, ok := asMap(m["env"])
	if ok {
		stripHasValueKeys(env)
		m["env"] = env
	}
	headers, ok := asMap(m["headers"])
	if ok {
		stripHasValueKeys(headers)
		m["headers"] = headers
	}
}
