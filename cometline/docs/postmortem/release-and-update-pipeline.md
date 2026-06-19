# Release and update pipeline bugs (packaged 404 + draft blocks auto-update)

**Date:** 2026-06-14  
**Components:** `electron/main.cjs`, `electron/preload.cjs`, SvelteKit static adapter, `electron-updater`, `.github/workflows/release.yml`

## Symptom A: Packaged app 404 after update install

After installing an update, the packaged app could show a plain `404 Not Found` page and repeatedly log `ERR_CONNECTION_REFUSED` for `http://127.0.0.1:7700/api/v1/health`.

The renderer console also showed:

```text
Not found: /index.html
```

## Symptom B: Release stuck as draft with partial assets blocks auto-update

The auto-updater never found new versions. On GitHub, the releases were stuck as **drafts** with only a single tiny asset (`*.blockmap`, ~153 KB) — the `.dmg`, `.zip`, and `latest-mac.yml` were all missing.

## Root causes

### A1. Packaged window loaded `app://bundle/index.html` directly

The custom protocol correctly served the static file, but SvelteKit saw the browser location as `/index.html`. Since the SPA has no `/index.html` route, the client router rendered SvelteKit's 404 page inside the app shell.

### A2. Updater quit path intercepted by macOS hide-on-close

We set `relaunchForUpdate = true`, stopped CometMind, then called `autoUpdater.quitAndInstall()`. But `stoppingForQuit` stayed false, so the `BrowserWindow` `close` handler still treated the updater quit like a normal macOS close and hid the window instead of letting the app exit.

### B1. electron-builder `--publish always` creates drafts

The release job ran `electron-builder --mac --publish always`, which uploads to GitHub in a fire-and-forget manner and creates the release as a **draft**. Drafts are invisible to electron-updater — the client reads `latest-mac.yml` from the _published_ release.

### B2. Large uploads did not complete

CI logs showed electron-builder start uploading the dmg/zip, but the next workflow step was already running ~18s later. A 147 MB dmg and 144 MB zip cannot upload in ~18s — the process exited before uploads finished, leaving a partial draft.

### B3. Stale-draft cleanup missed untagged releases

The cleanup step filtered releases by `.tag_name == TAG`, but electron-builder drafts have no tag attached yet (they appear as `untagged-*`), so orphaned partial drafts accumulated.

## Fix

### A: Packaged 404 and quit interception

- Load packaged builds at `app://bundle/` instead of `app://bundle/index.html`, so SvelteKit's route is `/`.
- Set `stoppingForQuit = true` before `quitAndInstall()` so the close handler does not hide the window during update installation.
- Set `autoInstallOnAppQuit = false`; updates should install only through the explicit Restart/Install action.

### B: Draft releases and partial uploads

Stop relying on electron-builder's `--publish always`. Make publishing an explicit, synchronous step:

- Build with **`build:mac`** (`electron-builder --mac --publish never`), which still produces every artifact plus `latest-mac.yml` in `cometline/dist/`.
- Verify the dmg/zip exist and are non-empty before publishing.
- Publish with **`gh release create "$TAG" --target "$GITHUB_SHA" --generate-notes <assets…>`**, which creates a **non-draft** release and waits for each asset upload to finish.
- Clean up both the tagged release for this tag **and** any `untagged-*` drafts whose assets match this version before building.
- Add a post-publish verification step that asserts `latest-mac.yml`, a `.dmg`, and a `-mac.zip` are attached to the release.

### Renderer/updater wiring

- `electron/main.cjs`: emit `cometline:update-state` (`checking`/`downloading`/`ready`/`error` with version + percent) to the renderer, plus IPC handlers.
- `electron/preload.cjs`: expose `getUpdateState`, `checkForUpdates`, `installUpdate`, `onUpdateState`.
- `UpdateButton.svelte`: bottom-left dock showing download progress and a "Restart to update" button when an update is ready.

## How to avoid regressions

### Packaged app

- Do not load `index.html` as the visible URL for static SvelteKit Electron bundles. Serve it as the fallback file, but load the app at the route URL (`/`).
- Any quit path that must truly terminate the app must set `stoppingForQuit` before window close events can fire.

### Release pipeline

- For electron-updater on GitHub Releases, the release **must be published (non-draft)** and contain `latest-mac.yml`, the `.zip`, and the `.blockmap`. Treat draft releases as broken for auto-update.
- Prefer an explicit `gh release create/upload` (which blocks until uploads complete) over tools that upload asynchronously and may exit early.
- When matching releases for cleanup, remember electron-builder drafts are `untagged-*` until published — match on assets/version, not just `tag_name`.
- The "Verify release assets uploaded" step fails the run if required assets are missing, so a partial release can no longer pass silently.

## Verification

1. Install an update via the in-app "Restart to update" button — app relaunches without 404.
2. Check GitHub Releases — release is published (not draft), contains `latest-mac.yml`, `.dmg`, `.zip`, and blockmaps.
3. Auto-updater detects new version when a published release exists.
4. CI run shows "Verify release assets uploaded" step passing.
