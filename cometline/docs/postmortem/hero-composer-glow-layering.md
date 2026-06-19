# Hero composer glow layering and animation

**Date:** 2026-06-14  
**Components:** `HeroComposerFrame.svelte`, `ChatView.svelte`, `+page.svelte`, `Composer.svelte`, `FirstTurnFlight.svelte`, `SettingsAppearancePanel.svelte`, `hero-composer-appearance.ts`, `settings.svelte.ts`, `app.css`

## Symptom

Several iterations of the hero composer aura failed to match the design intent:

1. **Glow on the wrong layer** — aura sat on an outer shell or on `Composer.svelte` pseudos, misaligned with the white hero card (wrong radius, too large).
2. **Wrong sequence** — border ring appeared before glow, or only after glow fully finished; design calls for glow to rise first, with border + card reaction **when the glow hits**, not after wrap completes.
3. **Lost “rise from bottom” feel** — after splitting ring and glow, the glow only faded in with `opacity`, with no upward motion.
4. **First-turn exit out of sync** — on first send, glow ran its own sink animation while `composer-wrapper` moved on a separate 560ms transition; aura and composer felt like two beats.
5. **Giant static wash at rest** — glow element was extended to viewport bottom at rest; radial gradients painted a full-screen wash instead of a tight halo.
6. **Hover mistaken for impact** — `hover:scale(1.01)` was added; intent was an **enter** reaction when glow hits the textarea, not a hover state.
7. **Hard-coded pink only** — no settings surface; default did not match the Arc-style light blue reference.

## Root cause

### 1. Layer ownership

Hero aura is **not** composer chrome. It lives in `HeroComposerFrame.svelte` wrapping `<Composer variant="hero" />`, mounted from `+page.svelte` and `ChatView.svelte`. Never put glow pseudos on `Composer.svelte` — dock variant must stay clean.

### 2. Animation geometry ≠ resting geometry

A broken fix extended glow `bottom` by `--hero-glow-travel` and kept `scaleY(1)` at rest, turning the halo into a tall background gradient. Correct model: **tight inset box at rest**, `translateY(travel)` only in enter keyframes.

### 3. Sequential timing felt like “after the hit”

Ring and scale used `--duration-hero-ring-delay: 0.55s` on a `0.65s` glow — they started near glow **completion**, so border and scale felt late. Design intent: **overlap** — glow still wrapping when border appears and card scales.

### 4. Competing motion on first turn

Instant `{#if active}` removal plus independent glow `translateY` exit fought `.composer-wrapper`’s 560ms dock transition.

### 5. Colors only in CSS

`app.css` tokens were static; no persistence or presets until `appearance.heroComposer` was added to settings.

## Fix (applied)

### Architecture: `HeroComposerFrame.svelte`

Three synchronized enter tracks on one frame (glow + ring layers; scale on the frame):

| Track | Element | Enter timing |
| ----- | ------- | ------------ |
| Glow rise | `.hero-composer-glow` | 0 → 650ms: `translateY(travel)` + `scaleY(0.35→1)` |
| Border ring | `.hero-composer-ring` | **280ms → 650ms**: `clip-path` rise (overlaps glow wrap) |
| Impact scale | `.hero-composer-frame` | **280ms → 650ms**: `scale(1→1.01)` when glow “hits” |

Tokens in `app.css`:

```css
--duration-hero-sequence: 0.65s;
--duration-hero-glow-rise: 0.65s;
--duration-hero-hit-delay: 0.28s;
--duration-hero-ring-rise: 0.37s;
--duration-hero-impact-rise: 0.37s;
--hero-composer-impact-scale: 1.01;
--duration-hero-exit-ring: 0.24s;
```

`--duration-hero-hit-delay` is shared by ring and impact — do **not** bring back a late `--duration-hero-ring-delay` that waits for glow to finish.

### Enter: measure, then animate

- Glow box: `inset: -16px -12px -10px` (tight halo).
- `measureGlowTravel()` → `--hero-glow-travel` = `.chat-home` bottom − frame bottom.
- Gate animations with `glowReady` after measure (`ResizeObserver` on frame + `.chat-home`).
- `class:impact-ready={glowReady && active && !exiting}` drives frame scale enter.

### First-turn exit

```typescript
onPrepareFlight={() => {
  if (composerVariant === 'hero') heroFrameExiting = true;
  shellStore.dockComposer();
}}
```

- `active={composerVariant === 'hero' && !heroFrameExiting}`
- `exiting={heroFrameExiting}` keeps layers mounted until outro ends
- **Ring:** `clip-path` collapse (240ms)
- **Frame scale:** `1.01 → 1` (240ms) via `.hero-composer-frame.exit`
- **Glow:** opacity fade only over `--duration-flight` (560ms) — position from `.composer-wrapper` transition, no independent sink

See also [composer-phase-and-positioning.md](./composer-phase-and-positioning.md) for dock / flight coordination.

### Configurable colors (Settings → Hero glow)

Persisted in `cometline-settings.json` as `appearance.heroComposer`:

| Preset | Glow | Border | Notes |
| ------ | ---- | ------ | ----- |
| **Blue** (default) | `#72c0ff` | `#9ed8ff` | Arc-style light blue |
| **Rose** | `#f43f5e` | `#fb7185` | Original pink |

- `hero-composer-appearance.ts` — presets, normalize, `heroComposerCssVars()`
- `+layout.svelte` — applies CSS vars from `settingsStore` on load / save
- Custom hex still supported; UI shows **Custom** when colors don’t match a preset

## How to avoid regressions

- **Do not put hero aura on `Composer.svelte`.**

- **Do not extend glow `bottom` by `--hero-glow-travel` at rest.** Use `translateY(travel)` in enter keyframes only.

- **Hit timing:** ring + `scale(1.01)` start at `--duration-hero-hit-delay`, **during** glow rise — not after `--duration-hero-glow-rise` completes. Tuning “when it hits” = adjust hit delay, not ring delay after glow end.

- **Impact is enter animation, not `:hover`.** No `hover:scale` on the frame.

- **First-turn exit:** one motion source for position — `composer-wrapper` + `--duration-flight`. Glow fades; frame scale resets; no second vertical glow animation.

- **Measure before animate:** `glowReady` required before `.ready` / `.impact-ready` classes.

- **Settings defaults:** new installs and “Reset defaults” use **Blue** preset; existing saved pink settings are preserved until user changes them.

- **Home vs session:** travel distance differs (grid vs absolute hero). Always measure from DOM.

## Verification

1. **Home (`/`)** — refresh: glow rises from UI bottom; at ~280ms border + slight scale start while glow still wrapping; all settle by ~650ms.
2. **Halo at rest** — tight around card, not a page-wide wash; default blue unless settings say otherwise.
3. **Settings → Hero glow** — Blue / Rose presets apply; custom hex shows “Custom”; Save updates live glow.
4. **First send** — scale returns to 1, ring collapses, glow fades while composer docks with flight (~560ms); no lagging sink.
5. **Docked / existing session** — no glow; no flash on load.
6. **Resize** — re-measure travel; remount replays enter.
7. **`prefers-reduced-motion`** — final glow + ring + scale state; exit clears without keyframe wait.
