# Provider label casing inconsistency ("OpenAI Compatible" vs "OpenAI-compatible")

**Date:** 2026-06-23  
**Components:** `settings/schema.ts`, `SettingsPanel.svelte`, `SettingsProvidersPanel.svelte`, `SetupWizard.svelte`

## Symptom

The OpenAI-compatible built-in provider displayed different names depending on
which UI surface rendered it:

- **Settings sidebar / provider name** (sourced from `DEFAULT_PROVIDERS` via
  `schema.ts`): **OpenAI Compatible** (capital C).
- **Method dropdown, METHOD_LABELS fallback, and the setup wizard** (sourced
  from per-component `METHOD_LABELS` maps): **OpenAI-compatible** (lowercase c,
  hyphenated style).

A user selecting the OpenAI-compatible provider in the setup wizard saw
"OpenAI-compatible", while the same provider appeared as "OpenAI Compatible"
in the settings sidebar. The disconnect made the wizard feel like it was
describing a different provider than the one configured in Settings.

## Root cause

The provider display name lives in **two independent sources** that drifted:

1. **`schema.ts` — canonical names.** `BUILTIN_PROVIDER_NAMES` (line 32) and
   `DEFAULT_PROVIDERS` (line 227) both use `'OpenAI Compatible'` (capital C,
   space-separated). This is the name persisted to
   `~/.cometmind/cometline-settings.json` and shown wherever `provider.name`
   is read directly.

2. **Per-component `METHOD_LABELS` maps — display fallbacks.** Three
   components define their own `Record<ProviderMethod, string>` maps for
   rendering the method label when `provider.name` is empty or when showing
   the method independently of the provider:
    - `SettingsPanel.svelte:66`
    - `SettingsProvidersPanel.svelte:13` (+ method `<option>` at line 167)
    - `SetupWizard.svelte:16`

    All three used `'OpenAI-compatible'` (lowercase c), copied from each other
    rather than from the canonical source.

There is no single shared label constant. Each component re-declares its own
`METHOD_LABELS`, so a casing change in `schema.ts` does not propagate. The
hyphenated lowercase style was likely chosen to mirror the raw method enum
value (`'openai-compatible'`), while the canonical name followed
title-case conventions.

## Fix

Standardize all `METHOD_LABELS` entries on the canonical title-case form
`'OpenAI Compatible'` (capital C), matching `BUILTIN_PROVIDER_NAMES` and
`DEFAULT_PROVIDERS` in `schema.ts`:

- `SettingsPanel.svelte:66` — `'OpenAI-compatible'` → `'OpenAI Compatible'`
- `SettingsProvidersPanel.svelte:13` — same
- `SettingsProvidersPanel.svelte:167` (method `<option>`) — same
- `SetupWizard.svelte:16` — same

## How to avoid regressions

- **Single source of truth for labels.** `METHOD_LABELS` should not be
  re-declared per component. Extract a shared `PROVIDER_METHOD_LABELS`
  constant (or reuse `BUILTIN_PROVIDER_NAMES`) from `schema.ts` and import it
  wherever a method display label is needed. This eliminates the copy-drift
  entirely.
- **When adding a new provider method**, update the canonical name in
  `BUILTIN_PROVIDER_NAMES` and `DEFAULT_PROVIDERS` only. If a component still
  has its own label map, the build will not catch the omission — grep for
  `METHOD_LABELS` to find every copy.
- **Prefer `provider.name` over `METHOD_LABELS[provider.method]`** in display
  code. The `name` field is what the user sees and edits in Settings; the
  method label is a fallback for empty names. The wizard and settings panel
  already do this via `provider.name || METHOD_LABELS[provider.method]`, so
  the label map only matters for the method dropdown and the empty-name edge
  case.
