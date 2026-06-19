# Fetch models fails with "An object could not be cloned"

**Date:** 2026-06-16  
**Components:** `SettingsPanel.svelte`, `settings.svelte.ts`, `electron/preload.cjs`, `electron/main.cjs`

## Symptom

In **Settings → Providers**, clicking **Fetch models** for an OpenAI-compatible
provider immediately failed with a red footer error:

> An object could not be cloned.

No network request reached the provider. The models list stayed on "No models
loaded yet."

## Root cause

**Fetch models** crosses the Electron renderer → main boundary:

1. `SettingsPanel.svelte` calls `settingsStore.fetchModelsFor(selectedProvider)`.
2. `fetchModelsFor` invoked `window.electronAPI.fetchProviderModels(provider)`.
3. Preload forwards that object with `ipcRenderer.invoke('cometline:fetch-provider-models', config)`.
4. Electron serializes IPC arguments with the **structured clone** algorithm.

`selectedProvider` is a Svelte 5 **`$derived`** value over `$state` draft
settings. Those objects are **reactive proxies**, not plain POJOs. Structured
clone rejects proxies and throws `DataCloneError` ("An object could not be
cloned") before the main process runs `fetchOpenAIModels`.

**Save settings** did not hit this path because `save()` runs
`normalizeSettings(draft)` first, which builds fresh plain objects. Only fetch
passed the live reactive reference straight into IPC.

## Fix

Clone the provider to a plain object before IPC in `fetchModelsFor`:

```ts
const models =
  (await window.electronAPI?.fetchProviderModels?.(cloneProvider(provider))) ?? [];
```

`cloneProvider` already existed in `settings.svelte.ts` for local copies; reuse
it at the IPC boundary.

## How to avoid regressions

- Treat **every `ipcRenderer.invoke` / `send` argument** as structured-clone
  payload. Only pass JSON-like data: strings, numbers, booleans, plain objects,
  and arrays of the same.
- Never pass Svelte 5 **`$state` / `$derived` / `$state.raw` proxies** (or
  class instances, functions, DOM nodes, etc.) over Electron IPC.
- When adding new IPC from stores or components, either call an existing
  `clone*` / `normalize*` helper or use `structuredClone()` on a plain snapshot
  in development to catch non-cloneable fields early.
- If fetch fails **after** this fix, expect a real provider error (HTTP status,
  auth, wrong base URL) — not the clone message.

## Follow-up (2026-06-16): timeout after clone fix

Once the IPC clone bug was fixed, fetch could reach the main process but still
fail with:

> Error invoking remote method 'cometline:fetch-provider-models': TimeoutError: The operation was aborted due to timeout

### Cause

`fetchOpenAIModels` used `AbortSignal.timeout(12000)` (12 s). Slow or
unreachable corporate gateways often exceed that. A second issue: model URL
construction appended `/models` directly to the base URL, while chat
completions use comet-sdk's `providerbase.Endpoint` logic (`/v1/models` when
the base URL does not already end in `/v1`). A base like
`https://api.example.com` therefore hit `/models` instead of `/v1/models`,
which can hang or never respond.

### Fix

- Align model discovery URLs with `openAICompatibleEndpoint()` (same rules as
  comet-sdk).
- Raise the fetch timeout to 30 s and surface the requested URL in timeout
  errors so users can verify VPN, base URL, and `/v1/models` reachability.
