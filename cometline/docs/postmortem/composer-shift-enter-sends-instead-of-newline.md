# Composer Shift+Enter sends instead of inserting newline

**Date:** 2026-06-16  
**Components:** `Composer.svelte`, `RichComposerInput.svelte`, `keyboard-shortcuts.ts`, `settings.svelte.ts`, `SettingsShortcutsPanel.svelte`, `electron/main.cjs`

## Symptom

After adding an **Insert newline in composer** shortcut (default **⇧ Enter**), pressing Shift+Enter in the message composer still **sent the message** instead of inserting a line break.

Plain Enter correctly sent; Shift+Enter did not behave like a multiline input.

## Root cause

Three gaps combined:

### 1. Legacy send binding matched any Enter variant

The default send shortcut is `{ key: 'Enter', shift: false }`, but saved settings often had the older shape `{ key: 'Enter' }` with **no `shift` field**.

In `matchesShortcut`, modifier checks are skipped when a field is `undefined`:

```ts
if (binding.shift !== undefined && binding.shift !== event.shiftKey) return false;
```

So a bare `{ key: 'Enter' }` binding matched **both** Enter and Shift+Enter. Users who had customized or persisted shortcuts before `shift: false` was recorded kept the overly broad binding.

### 2. Send was checked before newline

`Composer.svelte` handled `sendMessage` before `insertNewline`. When the legacy send binding matched Shift+Enter, the handler called `submit()` and returned — the newline shortcut never ran.

### 3. Shortcut capture did not record `shift: false`

`captureShortcut` only set `shift: true` when Shift was held. Recording plain Enter in Settings saved `{ key: 'Enter' }` without an explicit `shift: false`, reintroducing the ambiguous binding on the next save.

## Fix

1. **`keyboard-shortcuts.ts`**
    - Added `insertNewline` with default `{ key: 'Enter', shift: true }`.
    - `normalizeComposerEnterBinding` migrates legacy bare Enter send bindings to `{ key: 'Enter', shift: false }`.
    - `matchesShortcut` rejects Shift+Enter for bare Enter bindings even before migration runs.
    - `captureShortcut` records `shift: false` when Enter is captured without Shift.

2. **`Composer.svelte`**
    - Check `insertNewline` **before** `sendMessage`.
    - On match: `preventDefault()` and `input.insertText('\n')`.

3. **`electron/main.cjs`**
    - Added `insertNewline` to default shortcuts so Electron-side settings stay aligned.

## How to avoid regressions

- **Enter-family shortcuts must pin Shift explicitly** — `{ key: 'Enter' }` is ambiguous; always store `shift: true` or `shift: false` for composer Enter bindings.
- **Check newline before send** in the composer handler so Shift+Enter wins if bindings overlap during migration or user misconfiguration.
- **Normalize saved shortcuts on load** — `normalizeKeyboardShortcuts` runs in `settings.svelte.ts`; add migration helpers there for legacy binding shapes, not only in the UI defaults.
- **When adding a settings shortcut**, update `SHORTCUT_DEFINITIONS`, `types.ts`, Electron defaults in `main.cjs`, and add a `keyboard-shortcuts.test.ts` case for modifier distinction.

## Verification

1. Settings → Keyboard shortcuts shows **Send message** = `Enter` and **Insert newline in composer** = `⇧ Enter`.
2. Type in the composer → **Enter** sends, **Shift+Enter** inserts a visible line break (multiline input).
3. With legacy saved settings `{ sendMessage: { key: 'Enter' } }` (no shift field) → reload app → Shift+Enter inserts newline; Enter still sends.
4. Re-bind Send to Enter in Settings → saved binding includes `shift: false`.
