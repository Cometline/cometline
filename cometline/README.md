# Cometline

A local-first desktop AI assistant with persistent workspace sessions, live streaming reasoning and tool visibility, semantic memory, and a native-feeling macOS UI — while keeping the trusted agent runtime outside the renderer.

This directory is one module inside the `cometline-release` monorepo. The historical standalone `cometline` repo is archived; current development, issues, and pull requests land in the monorepo root.

Cometline is the **desktop shell** in the Cometline stack:

```
cometline   →  Electron + SvelteKit UI, sidecar lifecycle, settings, animations
cometmind   →  local agent runtime, SQLite, HTTP/SSE API, tools, memory, ACP
comet-sdk   →  provider-agnostic LLM I/O
```

The rule: **Cometline is not the brain.** CometMind is the brain. Comet SDK is only the model I/O boundary.

---

## Features

### Chat & sessions

- Workspace-scoped session list with search and delete confirmation
- Hero composer on `/` for new chats; docked composer on `/session/[id]`
- First-turn flight animation from hero to thread
- Live streaming of assistant text, reasoning blocks, and tool calls/results
- Subagent delegation UI (ACP progress, awaiting-input prompts)
- Message queue while a turn is in flight
- Stop streaming (⌘C or composer stop button)
- Per-session model picker grouped by provider
- Multimodal input: paste/drop up to 6 images (PNG/JPEG/GIF/WebP)
- Drop text files as fenced code blocks in the composer
- Slash commands: `/change`, `/create-skill`, `/model`, plus dynamic `/skill-name` menu from CometMind skills

### Markdown & links

- Rich assistant rendering: Shiki syntax highlighting, KaTeX math, GFM tables
- URL embed chips with favicons in composer and messages
- In-app **web panel** (⌘⌥B / ⌘O) for http(s) links; external mailto opens in the system browser

### Memory

- Settings panel for memory config, CRUD, semantic search, and compaction
- In-thread display of retrieved memories during reasoning
- Post-turn memory update hints on assistant messages

### Settings

- **Providers** — ChatGPT Codex, OpenAI, Anthropic, OpenAI-compatible, OpenCode Go, plus custom providers; fetch models from the provider API where available
- **CometMind** — ACP (OpenCode) config, Agent Skills management, Discord gateway toggle and config
- **Memory** — auto retrieve/extract, thresholds, embedding model, compaction
- **General** — open at login (macOS); session retention, max sessions per workspace, archived memory purge
- **Hero glow** — composer glow/border presets and custom colors; caret trail animation
- **Shortcuts** — rebind all keyboard shortcuts

### Desktop shell

- Electron sidecar spawns `cometmind serve` on `127.0.0.1:7700`
- macOS hidden title bar with animated traffic-light positioning when the sidebar toggles
- Menu bar tray icon; close button hides instead of quitting
- GitHub auto-update (check on launch + every 4 hours; manual install)
- First-run intro animation (replayable from Settings → About)
- Workspace picker (default `~/Cometline`)

---

## Quick start

From the monorepo root:

```bash
make install   # pnpm install in cometline/
make dev       # build CometMind sidecar + launch Electron dev app
```

You can still run frontend-only commands from `cometline/`, but day-to-day contributor workflow now starts from the monorepo root.

On first launch, open **Settings** (⌘,) to enable a provider, add credentials if needed, fetch or enter models, and choose default model roles. Settings are saved to `~/.cometmind/cometline-settings.json` (CometMind reads the same file).

Optional one-off provider overrides:

```bash
COMETMIND_PROVIDER=openai \
COMETMIND_MODEL=gpt-4o \
COMETMIND_BASE_URL=https://api.openai.com/v1 \
COMETMIND_API_KEY='...' \
make dev
```

Never commit real API keys.

---

## Keyboard shortcuts

Default bindings (rebindable in Settings → Shortcuts):

| Action | macOS default |
|---|---|
| Toggle sidebar | ⌘B |
| Open settings | ⌘, |
| New chat | ⌘T |
| Stop response | ⌘C |
| Send message | Enter |
| Insert newline | ⇧Enter |
| Focus search | ⌘F |
| Previous / next chat | ⌃⌘↑ / ⌃⌘↓ |
| Toggle web panel | ⌘⌥B |
| Open web panel | ⌘O |

---

## Project layout

```
cometline/
├── electron/
│   ├── main.cjs          sidecar lifecycle, IPC, settings, updater, tray
│   └── preload.cjs       contextBridge -> window.electronAPI
├── src/
│   ├── routes/           SvelteKit pages (/ and /session/[id])
│   └── lib/
│       ├── client/       CometMind REST/SSE client
│       ├── components/   AppShell, ChatView, Composer, Settings*, WebPanel, ...
│       ├── stores/       chat, session, settings, model, shell, runtime
│       └── reducers/     pure SSE → chat item reducer
├── buildResources/       app icon, tray icons, entitlements
├── static/               project avatar and icons for in-app UI
├── SOUL.md               default system prompt written into CometMind config
└── docs/
    └── COMETLINE_ARCHITECTURE.md
```

---

## Build & test

```bash
# From cometline/
pnpm run generate:api    # regenerate TS client from ../cometmind/openapi.yaml
pnpm run check          # svelte-check
pnpm run test           # vitest
pnpm run lint           # eslint
pnpm run build          # production renderer build
pnpm run build:electron # sidecar + renderer + electron-builder
pnpm run build:mac      # sidecar + renderer + electron-builder (no publish)
pnpm run release:mac    # build + publish to GitHub releases
```

From the monorepo root:

```bash
make check              # SDK + CometMind tests + svelte-check
make build              # SDK + CometMind binary + renderer
make package            # packaged Electron app with embedded sidecar
make port               # show process on 127.0.0.1:7700
make clean-log          # remove ~/.cometmind/cometline.log
```

Packaged apps embed the CometMind binary as an Electron extra resource and serve the renderer over a custom `app://` protocol.

---

## Runtime files

| Path | Purpose |
|---|---|
| `~/.cometmind/cometmind.db` | CometMind SQLite database |
| `~/.cometmind/cometline-settings.json` | Single settings file (providers, CometMind runtime, appearance, shortcuts) |
| `~/.cometmind/config.toml` | Legacy only; CometMind migrates from this if JSON is absent |
| `~/.cometmind/cometline-workspace.json` | Selected workspace path |
| `~/.cometmind/cometline.log` | Sidecar stdout/stderr (rotates at 10 MB while running → `.log.1`) |
| `~/.cometmind/cometline-gateway.log` | Discord gateway log (same rotation) |

---

## Architecture

See [`docs/COMETLINE_ARCHITECTURE.md`](docs/COMETLINE_ARCHITECTURE.md) for runtime contracts, data flow, IPC surface, and operational notes.

For a contributor-oriented map of the whole monorepo, see [`../ARCHITECTURE_GUIDE.md`](../ARCHITECTURE_GUIDE.md).

---

## Tech stack

| Layer | Choice |
|---|---|
| Desktop shell | Electron 42 |
| Renderer | SvelteKit 2 + Svelte 5 + TypeScript |
| Styling | Tailwind CSS v4 |
| Markdown | marked, Shiki, KaTeX, DOMPurify |
| Sidecar | CometMind Go binary on port 7700 |
| Packaging | electron-builder (macOS DMG + ZIP, notarized) |
