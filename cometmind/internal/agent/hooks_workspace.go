package agent

// HooksWorkspace returns the workspace root used for hook subprocess cwd, when hooks are configured.
func (r *Runner) HooksWorkspace() string {
	if r == nil || r.Hooks == nil {
		return ""
	}
	return r.Hooks.WorkspacePath()
}
