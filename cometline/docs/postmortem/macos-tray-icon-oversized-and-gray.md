# macOS tray icon oversized and gray

**Date:** 2026-06-16  
**Components:** `electron/main.cjs`, `buildResources/trayIcon.png`, `buildResources/trayTemplate.png`, `package.json`

## Symptom

The Cometline menu bar icon on macOS was visibly larger than the surrounding system tray icons and appeared as a gray/white blob instead of the project avatar.

After the 16×16 / 32×32 fix, a follow-up report showed the icon **too small** next to neighbors: the assets were regenerated from `static/project_avatar_96.png` (circular avatar on a square canvas), so at 16pt most of the slot was empty padding rather than artwork.

## Root cause

Two separate issues stacked together:

1. **Wrong `scaleFactor` after resize** — `nativeImage.resize()` leaves `scaleFactor` at `1.0`. A `32×32` or `44×44` bitmap is then interpreted as **32pt or 44pt on screen**, not 16pt. Menu bar neighbors use ~16pt logical size, so the icon looked roughly 2× too large (white circle filling the bar height).

2. **Template mask on colored artwork** — `trayTemplate.png` was a colored avatar, but macOS-only code always called `setTemplateImage(true)`. That turned the opaque avatar into a solid monochrome blob (gray/white circle).

An intermediate fix used `44×44` pixels thinking “22pt @2x”, but without `scaleFactor: 2.0` that still renders as **44pt** — even larger.

```javascript
// electron/main.cjs (before)
if (process.platform === 'darwin') {
	const isTemplateAsset = candidate.endsWith('trayTemplate.png');
	if (!isTemplateAsset) {
		image = image.resize({ width: 22, height: 22, quality: 'best' });
	}
	image.setTemplateImage(true); // always applied on macOS
	return image;
}
```

The `22×22` resize for non-template fallbacks was also inconsistent with the code comment that said 16pt / 32px Retina backing.

## Fix

### 1. Dedicated colored tray icon

Generated `buildResources/trayIcon.png` (32×32) from `static/project_avatar_96.png` so the menu bar shows a small, recognizable colored avatar instead of a template mask.

```bash
sips -z 32 32 static/project_avatar_96.png --out buildResources/trayIcon.png
```

### 2. Prefer the colored icon in candidate resolution

`resolveTrayIconCandidates()` now lists `trayIcon.png` first, with `trayTemplate.png` kept as a backward-compatible fallback.

### 3. Ship `trayIcon.png` (16×16) + `trayIcon@2x.png` (32×32)

Electron’s documented approach: load `trayIcon.png` by path; on Retina it automatically picks the `@2x` sibling. No manual `scaleFactor` re-wrap needed.

```bash
# Crop ~85% from the center of icon.png so the squircle fills the menu bar slot.
sips -c 850 850 buildResources/icon.png --out /tmp/tray-crop.png
sips -z 16 16 /tmp/tray-crop.png --out buildResources/trayIcon.png
sips -z 32 32 /tmp/tray-crop.png --out buildResources/trayIcon@2x.png
```

Do **not** use `static/project_avatar_96.png` for tray icons — the circular avatar leaves heavy padding at 16×16 and reads as a tiny dot in the menu bar.

Both files must be bundled (`package.json` `extraResources`) and live in the same folder in dev (`buildResources/`).

### 4. Only template actual template assets

`setTemplateImage(true)` is applied only for `trayTemplate.png`. `trayIcon.png` stays colored and is listed first in candidates.

### 5. Bundle the new asset

`package.json` `extraResources` now copies `buildResources/trayIcon.png` into the app bundle so the colored icon is available in packaged builds.

## If changing the tray icon later

| What to change           | File                                              | Notes                                                                                                                                                |
| ------------------------ | ------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| Colored tray source      | `buildResources/icon.png`                         | Center-crop ~850×850 from the 1024 squircle, then export 16×16 + 32×32. Avoid `project_avatar_96.png` (circular, too much padding at menu bar size). |
| Monochrome menu bar icon | `buildResources/trayTemplate.png`                 | Must be a true template (single color with alpha). Keep the `Template.png` suffix so `setTemplateImage(true)` is applied.                            |
| Candidate order          | `electron/main.cjs`                               | First-match wins; put the desired icon first.                                                                                                        |
| Menu bar icon size       | `buildResources/trayIcon.png` + `trayIcon@2x.png` | 16×16 + 32×32 pair; never a lone 32px file at scaleFactor 1.0.                                                                                       |
| Packaged asset inclusion | `package.json`                                    | Include both `trayIcon.png` and `trayIcon@2x.png` in `extraResources`.                                                                               |

## How to avoid regressions

- Do not name a colored image `*Template.png` and then call `setTemplateImage(true)` on it. macOS will treat it as a mask and render it as a monochrome blob.

- After `resize()`, always re-wrap with `createFromBuffer(..., { scaleFactor: 2.0 })` for 32×32 assets, or resize to 16×16 at scaleFactor 1.0. **Pixel size without scaleFactor is interpreted as point size.**

- Prefer the **`@1x` + `@2x` file pair** over programmatic `createFromBuffer` — the latter can create a tray that logs “ready” but renders invisible in dev.

- Do not use `44×44` unless you also set `scaleFactor: 2.0` (22pt) — standard menu bar icons are 16pt, not 22pt.

## Dev vs packaged (`make dev` vs release app)

Symptom: `make dev` logs `[tray] Menu bar icon ready` but nothing appears in the menu bar; the packaged `.app` shows an icon (sometimes oversized).

|                 | `make dev`                                                                   | Packaged release                                                      |
| --------------- | ---------------------------------------------------------------------------- | --------------------------------------------------------------------- |
| Binary          | `electron .` from `cometline/` (Dock shows **Electron**)                     | `Cometline.app` (Dock shows **Cometline**)                            |
| Tray assets     | `cometline/buildResources/` via `app.getAppPath()`                           | `Contents/Resources/` via `process.resourcesPath`                     |
| Typical failure | Tray object created with invisible bitmap (wrong scale / `createFromBuffer`) | Lone 32px or 44px asset at scaleFactor 1.0 → huge white template blob |

Tray is created at **startup** on macOS (not only when the window is hidden). If you only look after Cmd+W, the icon should already be in the menu bar while the window is open.

Dev log after fix should show: `[tray] Using …/buildResources/trayIcon.png { width: 16, height: 16 } [ 2 ]`.

### Menu bar crowded (macOS system behavior)

macOS can **silently hide** status bar items when the menu bar is full — even when empty space looks available. Smaller icons disappear first; oversized icons (e.g. old 44pt template blob) may still show. This is not an Electron bug in Cometline; it affects Fiddle tray demos too ([electron#45231](https://github.com/electron/electron/issues/45231)).

**Try:** quit other menu bar apps (Google Drive, VPN, etc.), or reduce items in **System Settings → Control Center → Menu Bar Only**. Then restart Cometline.

### Light menu bar + white tray artwork

If `trayIcon.png` was generated from a white circular avatar, it can be **invisible on a light menu bar** (white on white) while still logging “ready”. Regenerate from `buildResources/icon.png` (colored squircle) or use a true monochrome `*Template.png` asset.

- When adding a new tray icon, add it to both the dev candidate list and `package.json` `extraResources` so packaged builds can find it.

## Verification

1. Run the desktop app with `pnpm dev` on a Retina Mac; the menu bar icon should be the same size as neighboring system icons.
2. The icon should display in color, not as a gray/white silhouette.
3. Package the app (`pnpm build:mac`) and launch the `.app`; the menu bar icon should still be the colored avatar at the correct size.
4. Temporarily delete `buildResources/trayIcon.png`; the app should fall back to `trayTemplate.png` and render it as a smaller monochrome blob (not oversized).
5. `node --check electron/main.cjs` should pass.
