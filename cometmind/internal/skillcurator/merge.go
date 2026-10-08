package skillcurator

// MergeSources returns absorbed skill names that should be archived.
// Nothing is archived unless the surviving skill was actually written.
// The target and pinned skills are never archived.
func MergeSources(target string, absorbed []string, wrote bool, pinned map[string]bool) []string {
	if !wrote || target == "" {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, name := range absorbed {
		if name == "" || name == target || pinned[name] || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}
