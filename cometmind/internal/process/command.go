package process

import "time"

// treeWaitDelay bounds how long Wait blocks on pipes after the process tree
// is killed. A descendant that left the group can keep a pipe open; this is
// the backstop so the caller returns.
const treeWaitDelay = 2 * time.Second
