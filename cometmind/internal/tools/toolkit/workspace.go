package toolkit

import "github.com/Cometline/cometline/cometmind/internal/tools/fs"

// Workspace is the FileWorkspace module (owned by tools/fs).
// Tools are thin adapters over this seam.
type Workspace = fs.Workspace

// RuntimePrefix is the @runtime mount alias.
const RuntimePrefix = fs.RuntimePrefix
