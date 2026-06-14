# Packaged app 404 after update install

**Date:** 2026-06-14  
**Components:** `electron/main.cjs`, SvelteKit static adapter, `electron-updater`

## Symptom

After installing an update, the packaged app could show a plain `404 Not Found` page and repeatedly log `ERR_CONNECTION_REFUSED` for `http://127.0.0.1:7700/api/v1/health`.

The renderer console also showed:

```text
Not found: /index.html
```

## Root Cause

1. **The packaged window loaded `app://bundle/index.html` directly.** The custom protocol correctly served the static file, but SvelteKit saw the browser location as `/index.html`. Since the SPA has no `/index.html` route, the client router rendered SvelteKit's 404 page inside the app shell.

2. **The updater quit path was intercepted by macOS hide-on-close.** We set `relaunchForUpdate = true`, stopped CometMind, then called `autoUpdater.quitAndInstall()`. But `stoppingForQuit` stayed false, so the `BrowserWindow` `close` handler still treated the updater quit like a normal macOS close and hid the window instead of letting the app exit. That left CometMind stopped while the renderer kept polling health, producing `ERR_CONNECTION_REFUSED`.

## Fix

- Load packaged builds at `app://bundle/` instead of `app://bundle/index.html`, so SvelteKit's route is `/`.
- Set `stoppingForQuit = true` before `quitAndInstall()` so the close handler does not hide the window during update installation.
- Set `autoInstallOnAppQuit = false`; updates should install only through the explicit Restart/Install action.

## How to Avoid Regressions

- Do not load `index.html` as the visible URL for static SvelteKit Electron bundles. Serve it as the fallback file, but load the app at the route URL (`/`).
- Any quit path that must truly terminate the app must set `stoppingForQuit` before window close events can fire.
