# Hello Hooks example

Install as a user plugin:

```bash
mkdir -p ~/.cometmind/plugins/hello-hooks
cp examples/plugins/hello-hooks/plugin.json examples/plugins/hello-hooks/hook.sh ~/.cometmind/plugins/hello-hooks/
chmod +x ~/.cometmind/plugins/hello-hooks/hook.sh
```

Enable hooks in **Settings → CometMind → Tool hooks**, then send a turn that calls `run_command`.
The example PreToolUse handler prefixes commands with `rtk` unless already prefixed.

Project-scoped hooks under `{workspace}/.cometmind/` require adding the workspace path to **Trusted workspace paths** (same trust bar as `run_command`).

Hook contract: event JSON on **stdin**, outcome JSON on **stdout** (`block`, `modify_tool_input`, `append_system`). Failures are fail-open.
