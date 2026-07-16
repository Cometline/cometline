---
name: setup-cometline
description: Help configure Cometline providers, model defaults, skills, delegation, memory, storage, and gateways.
---

# Setup Cometline

Use this skill when the user wants help configuring Cometline or CometMind. This includes initial setup, provider API keys, model selection, default model roles, Agent Skills, coding-harness delegation, Discord gateway settings, memory, storage, import/export, or troubleshooting a broken local setup.

## Workflow

1. Identify what the user wants to configure before changing anything. If the request is ambiguous, ask one focused question.
2. Prefer the Settings UI for user-facing configuration instructions. Mention exact sections such as Settings -> Providers, Settings -> CometMind -> Skills, Settings -> CometMind -> Coding task delegation, Settings -> CometMind -> Discord, or Settings -> CometMind -> Storage.
3. Prefer the settings tools when the agent should inspect or change settings itself:
   - Runtime file: `~/.cometmind/cometline-settings.json` (providers, cometmind.*, defaults)
   - Desktop file: `~/.cometmind/cometline-desktop.json` (appearance, shortcuts, app/persona) — **Electron only**; never edit via tools or shell
   - `list_settings` — editable paths, secret flags, and apply class (`reload` / `gateway` / `unsupported`)
   - `get_settings` — read the runtime settings file (or a dotted path subtree); secrets are redacted to `__REDACTED__` with `<field>_has_value`
   - `patch_settings` — deep-merge a JSON patch into the runtime settings file (mode `0600`), then auto-apply (in-place reload and/or gateway process restart)
   - Rejected via tools: `appearance`, `shortcuts`, `app`, `systemPromptPath`, host/port — tell the user to use the Settings UI
   - When patching secrets, send a new string to set the key, or `__REDACTED__` / `***` to keep the previous value. Never ask the user to paste secrets into chat when a safer path exists.
4. CLI fallbacks remain available (`cometmind settings path|show|export|import|reload`, `cometmind process status|stop|restart`) when tools are unavailable. If `cometmind` is not found, try `~/.cometmind/bin/cometmind`.
5. After changes, almost all runtime fields hot-reload without killing the current chat turn. Changes under `cometmind.gateway` recycle **gateway process(es) only**. Full main-sidecar restart is reserved for true process-level bind changes (host/port) via the Settings UI.

## Provider Setup

Help the user choose a provider method, base URL, API key, enabled models, selected model, and default model roles. Environment variables such as `COMETMIND_PROVIDER`, `COMETMIND_BASE_URL`, `COMETMIND_API_KEY`, `COMETMIND_MODEL`, `ANTHROPIC_API_KEY`, and `OPENAI_API_KEY` can override runtime behavior, so check them when settings appear correct but runtime behavior differs.

## Skills Setup

CometMind discovers skills from configured roots, `~/.cometmind/skills`, global `~/.agents/skills`, workspace `.agents/skills`, workspace `.claude/skills`, and optional OpenCode or Claude skill roots. User-created skills should normally live under `~/.cometmind/skills/{skill-name}/SKILL.md`, global `~/.agents/skills`, or the workspace `.agents/skills` directory.

Use `/create-skill` when the user wants the agent to create a reusable skill. The agent should use the `write_skill` tool rather than editing skill files manually when possible.

## Delegation Setup

Coding-harness delegation is configured under Settings -> CometMind -> Coding task delegation. Use it when the user wants CometMind to hand coding work to OpenCode, Claude Code, or Codex. The UI exposes only the harness selector; command paths, arguments, permissions, and working-directory behavior are fixed by CometMind.

## Discord Gateway

Discord gateway configuration lives under Settings -> CometMind -> Discord. Confirm the bot token environment variable name, workspace path, allowed user IDs, and mention requirements. Do not ask the user to paste bot tokens into chat unless there is no safer option. Agent `patch_settings` on `cometmind.gateway` restarts gateway process(es) only — the main CometMind serve process (and the current chat turn) stays up.

## Memory And Storage

Memory and storage settings live under Settings -> CometMind. Explain what will be stored locally, what may be archived, and how provider/model choices affect extraction or summarization if the user is tuning memory behavior. Memory, storage cleanup interval, jobs reconcile interval, and autonomy changes apply via in-place reload.

## Troubleshooting

If settings do not persist, check `~/.cometmind/cometline-settings.json` and `~/.cometmind/cometline-desktop.json` plus file permissions. If the sidecar will not start, check `~/.cometmind/cometline.log`, port `7700`, and the packaged or development CometMind binary path. Note: `Reload` reconnects MCP servers, so in-flight MCP tool calls can fail even though the chat turn continues.
