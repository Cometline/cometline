# Settings And Persistence

This guide explains the rules behind the settings modal, the shared desktop
settings file, and the save flow across renderer, Electron, and CometMind.

This area is easy to break because the UI is draft-based, some controls save
immediately, some require an explicit Save, and some state is shared with the
runtime while other state is desktop-only.

## Source Of Truth

Current source of truth:

- `~/.cometmind/cometline-settings.json`

Legacy fallback only:

- `~/.cometmind/config.toml`

Current truth in code:

- Electron reads and writes `cometline-settings.json`
- CometMind loads JSON first, then falls back to legacy TOML only if JSON is missing

Relevant files:

- `cometline/electron/main.cjs`
- `cometmind/internal/config/config.go`
- `cometline/src/lib/stores/settings.svelte.ts`

Rule:

- Do not write new docs or features as if `config.toml` is still the primary config path.

## Persistence Categories

There are three persistence modes in the settings area.

### 1. Pending-save settings

These live in the modal draft until the user clicks `Save changes`.

Examples:

- provider config and enabled models
- default model roles
- most `cometmind` runtime config
- appearance settings like hero glow and caret trail
- app icon variant
- mini window inactivity timeout
- storage and retention settings
- memory retrieval, lifecycle, and embedding settings

### 2. Instant-save settings

These persist immediately without using the pending-save button.

Examples:

- shortcuts
- `app.openAtLogin`
- Discord gateway enabled/run toggle
- web panel width
- intro/setup completion flags

### 3. Action-based settings operations

These are not draft-vs-save fields. They are explicit commands.

Examples:

- fetch provider models
- Codex sign-in
- memory create/delete/search
- memory compaction preview/run
- import/export settings
- workspace selection and cleanup

## The Draft Model

The settings modal is draft-based.

Key behavior:

- `SettingsPanel.svelte` clones persisted settings into a local draft.
- Editing controls usually mutates only the draft.
- Closing the modal discards unsaved draft edits.
- Saving replaces persisted settings and then resets the draft from saved data.

Important files:

- `cometline/src/lib/components/settings/SettingsPanel.svelte`
- `cometline/src/lib/components/settings/settings-panel-controller.svelte.ts`
- `cometline/src/lib/settings/settings-draft.ts`

Rule:

- Do not quietly persist draft-only fields during ordinary input changes.

## Dirty-State Rules

Dirty state is snapshot-based, not flag-based.

Key file:

- `cometline/src/lib/settings/pending-settings.ts`

Important behavior:

- `pendingSettingsSnapshot(settings)` builds a normalized JSON snapshot.
- It intentionally excludes instant-save fields from pending-save dirty logic.
- `settingsPendingDirty(draft, persisted)` compares those snapshots.
- `sectionPendingDirty(...)` drives per-section visual indicators.
- `SECTION_PERSISTENCE_HINTS` documents which section fields are pending, instant, or action-based.

Current exclusions from pending-save dirty detection:

- `shortcuts`
- `app.openAtLogin`
- `cometmind.gateway.discord.enabled`

Rule:

- When adding a settings field, decide explicitly whether it is pending-save, instant-save, or action-based, then update the snapshot rules to match.

## The `isDirty()` Guardrail In Memory Settings

This is the most important postmortem rule in the settings UI.

Key file:

- `cometline/src/lib/components/settings/SettingsMemoryPanel.svelte`

The invariant:

- `isDirty()` is consumed from a `$derived` path.
- `isDirty()` and everything it calls must be pure reads of reactive state.
- It must not mutate `$state`.

Why:

- Mutating reactive state during a `$derived` evaluation breaks Svelte 5 dependency tracking.
- That regression previously caused embedding changes to stop re-triggering the Save button state, leaving `Save changes` disabled.

Safe pattern:

- `computeEmbeddingPayload(base)` is pure and can be used by `isDirty()`.
- `applyEmbeddingSelection()` is impure and must be used only in save-time code paths.
- `buildSavePayload()` is allowed to call the impure helper because it runs during an explicit save action, not during dirty-state derivation.

Rule:

- Never call `buildSavePayload()` or any mutating helper from `isDirty()` or from anything used by `$derived` dirty-state computation.

## Cross-Layer Save Flow

### Renderer load

`settingsStore.load()`:

- prefers `window.electronAPI.getProviderSettings()`
- falls back to `localStorage` in browser/dev contexts
- normalizes settings and updates `modelStore`

Key file:

- `cometline/src/lib/stores/settings.svelte.ts`

### Renderer save orchestration

`settings-panel-controller.svelte.ts` owns the modal save flow.

Important behavior:

- syncs panel-owned fields first with `syncFields()`
- preserves current UI selection state across save
- branches for memory-section save semantics
- computes `restartCometMind` via `providersNeedRestart(...)`, `cometmindNeedsRestart(...)`, and icon-variant diffing
- delegates to `settingsStore.save(...)`
- resets the draft from the saved result

Memory-section behavior:

- builds a memory API payload from the memory panel
- mirrors embedding choice into the shared settings draft
- persists shared JSON settings and live memory settings together

### Renderer persistence helper

`persistSettings(...)`:

- normalizes and validates settings
- saves through Electron IPC or browser `localStorage`
- optionally sends `PUT /api/v1/memory/settings`
- runs retention/session sync when the runtime is ready
- reconnects to CometMind when restart was requested

Key file:

- `cometline/src/lib/settings/persist.ts`

### Electron write path

Electron IPC handler `saveProviderSettings`:

- normalizes the incoming settings
- writes `~/.cometmind/cometline-settings.json`
- applies `0600` permissions
- refreshes global shortcuts
- optionally restarts CometMind
- syncs Discord gateway side effects
- applies open-at-login
- applies icon variant
- returns normalized saved settings back to the renderer

Key file:

- `cometline/electron/main.cjs`

### Backend load path

`cometmind/internal/config/config.go`:

- creates `~/.cometmind/` if needed
- loads `cometline-settings.json` when present
- falls back to legacy `config.toml` only if JSON is absent
- writes a minimal JSON settings file on first boot when neither exists
- applies env overrides and effective defaults

Rule:

- If you change shared settings shape or semantics, review renderer schema, Electron normalization, and backend loading together.

## Restart Rules

CometMind restart is required when:

- provider runtime settings changed
- `cometmind` runtime subtree changed
- icon variant changed and packaged runtime prompt path or related native state must be refreshed

CometMind restart is not required for:

- shortcuts
- open-at-login
- Discord gateway enabled toggle
- web panel width
- intro/setup flags

Operational detail:

- Electron waits for the sidecar to exit before respawning it to avoid port `7700` reuse and SQLite WAL lock issues.

Rule:

- Do not casually mark settings as restart-free. Match the existing restart helpers and native side effects.

## Other Persisted Files

Not everything lives in `cometline-settings.json`.

Also persisted:

- `~/.cometmind/cometline-workspace.json` - selected workspace path
- `~/.cometmind/cometmind.db` - runtime SQLite database
- `~/.cometmind/cometline.log` - CometMind sidecar log
- `~/.cometmind/cometline-gateway.log` - Discord gateway log

Rule:

- Before adding a new file on disk, confirm that the data cannot fit one of the existing storage paths.

## Memory Settings Nuance

There is one subtle area worth calling out.

Shared JSON persistence clearly covers:

- provider-linked embedding selection mirrored into frontend settings

Live runtime API persistence clearly covers:

- `PUT /api/v1/memory/settings`

But the full durability path for every memory retrieval/lifecycle runtime setting is less obvious than the main shared JSON path.

Practical rule for contributors:

- If you change memory settings, verify both immediate runtime behavior and restart-time durability.
- Do not assume that because a setting round-trips through the API, it is already persisted in the same way as shared JSON settings.

## Files To Read Before Changing Settings

- `cometline/src/lib/components/settings/settings-panel-controller.svelte.ts`
- `cometline/src/lib/components/settings/SettingsMemoryPanel.svelte`
- `cometline/src/lib/components/settings/settings-controller.svelte.ts`
- `cometline/src/lib/stores/settings.svelte.ts`
- `cometline/src/lib/settings/pending-settings.ts`
- `cometline/src/lib/settings/persist.ts`
- `cometline/electron/main.cjs`
- `cometmind/internal/config/config.go`

## Tests That Protect This Area

- `cometline/src/lib/settings/pending-settings.test.ts`
- `cometline/src/lib/components/settings/settings-controller.svelte.test.ts`
- `cometline/src/lib/settings/settings-save.test.ts`
- `cometline/src/lib/components/settings/SettingsMemoryPanel.svelte.test.ts`
- `cometmind/internal/config/config_test.go`
- `cometmind/server/memory_handlers_test.go`

Rule:

- Settings changes should usually come with an update to one of these tests, especially if the change touches dirty-state or restart semantics.
