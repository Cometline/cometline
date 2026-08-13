package tools

import "github.com/cometline/cometmind/internal/tools/fs"

// Workspace is the FileWorkspace module (owned by tools/fs).
// Tools are thin adapters over this seam.
type Workspace = fs.Workspace

// RuntimePrefix is the @runtime mount alias.
const RuntimePrefix = fs.RuntimePrefix

// MountDocs documents runtime mounts for coding policy prompts.
func MountDocs() string { return fs.MountDocs() }
