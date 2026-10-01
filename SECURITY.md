# Security Policy

## Reporting a vulnerability

Please report vulnerabilities privately through GitHub: open the [Security tab](https://github.com/Cometline/cometline/security) and choose **Report a vulnerability**. Do not open a public issue, pull request, or discussion for a security problem.

Include what you found, how to reproduce it, and the version or commit you tested. We aim to acknowledge reports within a week and will keep you updated in the advisory until a fix ships.

## Supported versions

Security fixes land on `main` and ship in the next release. Only the latest release is supported.

## Security model

Cometline is a local-first desktop app. These are the boundaries it relies on, which helps when judging whether something is a vulnerability.

- **Local API.** The CometMind sidecar serves its HTTP/SSE API on `127.0.0.1:7700`. It binds to loopback only and has no authentication, so it trusts every process running as the same user. CORS allows only local origins (`localhost`, `127.0.0.1`, `app://`, `file://`).
- **Renderer isolation.** Electron windows run with `sandbox`, `contextIsolation`, and no `nodeIntegration`. The renderer reaches native features only through the preload bridge (`window.electronAPI`).
- **API keys.** Provider API keys are stored in plain text in `~/.cometmind/cometline-settings.json`, written with mode `0600`. Keys can also come from environment variables (`COMETMIND_API_KEY`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`).
- **OAuth tokens.** MCP OAuth access and refresh tokens live in `~/.cometmind/mcp-oauth/{serverId}.json`, and the registered client identity in `{serverId}.client.json`, both mode `0600` and never in the settings file. ChatGPT Codex reuses the Codex CLI session in `~/.codex/auth.json`.
- **Agent tools.** File tools are sandboxed to the active workspace plus explicit runtime mounts such as `@runtime/wiki`. Shell commands (`run_command`) and delegated coding harnesses start in the workspace directory but run with the user's full permissions, and tool calls run without a confirmation prompt.

Escaping the workspace sandbox, reaching the local API from a web page, leaking keys or tokens outside the files above, and breaking renderer isolation are all in scope.
