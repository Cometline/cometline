# Frontend Design System

This guide describes the current Cometline frontend design system as it exists
today, not as an idealized future state.

Primary source files:

- `cometline/STYLING.md`
- `cometline/docs/FRONTEND_PATTERNS.md`
- `cometline/src/app.css`

## Core Styling Model

Cometline uses a three-layer styling model.

### 1. Global tokens and shared utilities

Source:

- `cometline/src/app.css`

This file owns:

- semantic color tokens
- layout tokens
- motion tokens
- shared utilities like `.content-panel-surface`, `.pane-focus-active`, `.spin`, and `.no-drag`

### 2. Scoped component CSS

Most real component styling belongs in local `<style>` blocks.

Use this for:

- component-specific layout
- local state selectors
- animation details
- interaction polish

### 3. Tailwind utilities

Tailwind is present, but secondary.

Use it for:

- simple layout tweaks
- quick spacing or flex utilities

Do not use it as a replacement for semantic tokens and component CSS.

## Non-Negotiable Styling Rules

From current project guidance:

- prefer `var(--token)` over hardcoded values
- do not use `@apply`
- do not mix semantic classes and Tailwind utilities on the same element
- add a token before introducing a new repeated semantic color
- keep styling colocated unless it is clearly a shared primitive

Source docs:

- `cometline/STYLING.md`
- `cometline/docs/FRONTEND_PATTERNS.md`

## Current Visual Language

Cometline's v1 visual identity is:

- light-theme-first
- soft white and warm-gray surfaces
- subtle borders and shadows
- rounded cards, pills, and chrome
- muted text hierarchy
- accent glow instead of loud brand blocks
- motion-heavy but generally reduced-motion aware

This is not a dark-theme-ready system today.

## Color System

Source of truth:

- `cometline/src/app.css`

Important token families:

- app and panel surfaces: `--app-bg`, `--panel-bg`, `--shell-canvas-bg`, `--sidebar-bg`
- text hierarchy: `--text-main`, `--text-muted`, `--text-soft`
- borders: `--border-soft`
- accent and state: `--accent`, `--status-success`, `--status-error`
- overlay/chrome: `--sidebar-overlay-bg`, `--overlay-scrim`

### Hero composer appearance

The hero composer glow is a first-class appearance system, not a one-off effect.

Key files:

- `cometline/src/lib/hero-composer-appearance.ts`
- `cometline/src/routes/+layout.svelte`

Current presets center on blue and rose variants and influence:

- hero composer aura
- pane focus ring/glow
- user bubble accents
- send/stop affordances

### Sidebar semantic families

`app.css` also defines color families for:

- pinned sections
- Discord-related sections
- workspace/session grouping

## Typography

Typography is mostly system-stack and restrained.

Base family:

- `-apple-system, BlinkMacSystemFont, 'Segoe UI', 'Helvetica Neue', Arial, sans-serif`

Common scale:

- `10px` to `12px` for metadata, chips, and labels
- `13px` to `15px` for main UI text
- `16px` to `20px` for modal and page headings

Common weights:

- `600` to `650` for headings, pills, and labels
- around `450` to `500` for normal UI copy

Rule:

- Stay near the existing scale unless you are changing a whole subsystem on purpose.

## Layout Tokens

Global layout tokens live in `app.css`.

Important examples:

- `--sidebar-width`
- `--web-panel-width`
- `--titlebar-height`
- `--traffic-light-gutter`
- `--chat-thread-width`
- `--chat-composer-width`
- `--composer-dock-bottom`
- `--thread-scroll-gap`

Rule:

- Prefer existing layout tokens before adding literal pixel values to components.

## Motion System

Cometline uses a mix of:

- CSS transitions
- CSS keyframes
- Svelte transitions
- RAF-driven bespoke animation

Global motion tokens include:

- `--ease-smooth`
- `--duration-fast`
- `--duration-medium`
- `--duration-flight`

Examples of motion-heavy components:

- `HeroComposerFrame.svelte`
- `IntroAnimation.svelte`
- `ThinkingIndicator.svelte`
- `WebPanel.svelte`
- `RichComposerInput.svelte`

Rule:

- Preserve reduced-motion behavior whenever adding or changing animation.

## Reusable Primitives

### Global/shared CSS

- `cometline/src/app.css`
- `cometline/src/lib/styles/chat-bubble.css`
- `cometline/src/lib/styles/fold-panel.css`
- `cometline/src/lib/styles/settings-panels.css`

### Settings primitives

- `SettingsButton.svelte`
- `SettingsToggle.svelte`
- `SettingsField.svelte`
- `SettingsSection.svelte`

### Chat/thread primitives

- `ThreadRow.svelte`
- `AssistantStack.svelte`
- `ThinkingBlock.svelte`
- `ToolFoldPanel.svelte`
- `MemoryCard.svelte`

### Composer and shell primitives

- `HeroComposerFrame.svelte`
- `Composer.svelte`
- `ComposerToolbar.svelte`
- `ContextWindowRing.svelte`
- `RichComposerInput.svelte`

Rule:

- Reuse these before inventing a new visual pattern in a nearby area.

## Current Consistency Gaps

These are known v1 realities, not necessarily blockers, but contributors should avoid making them worse.

### 1. Light-theme-only foundation

- The token system is not fully dual-theme.
- Syntax highlighting is explicitly light-theme oriented.

### 2. Hardcoded colors still exist

Despite the rules, some components still contain local color literals.

Examples include:

- `AssistantMarkdown.svelte`
- `RichComposerInput.svelte`
- `UpdateButton.svelte`
- `EmptyChatState.svelte`
- `onboarding/SetupWizard.svelte`

Rule:

- Prefer replacing or consolidating hardcoded colors when touching those areas.

### 3. Token vocabulary is incomplete

Some token names are referenced with fallbacks or partial usage patterns.

Examples surfaced during the audit:

- `--accent-soft`
- `--radius-pill`
- `--border-subtle`

Rule:

- If you rely on a semantic token more than once, define it centrally instead of repeating fallbacks.

### 4. Tailwind vs semantic-class mixing is not perfectly enforced

Some components drift from `STYLING.md` guidance.

Rule:

- New work should move toward the documented model, not away from it.

### 5. Component-local semantic colors

Some subsystems define local color recipes instead of shared semantic tokens.

Examples:

- skill/file chips
- markdown callouts
- some dialog states

Rule:

- When extending an existing local recipe into a broader pattern, promote it into shared tokens.

## Recommended Workflow Before Frontend Changes

Read these first:

1. `cometline/STYLING.md`
2. `cometline/docs/FRONTEND_PATTERNS.md`
3. `cometline/src/app.css`
4. the nearest shared CSS primitive file if one exists
5. the local component you plan to change

Then ask:

- Is there already a token for this color, spacing, or motion?
- Is there already a primitive for this card, button, fold panel, or row?
- Am I mixing Tailwind and semantic CSS in a way the repo discourages?
- Will this still behave well under reduced motion and narrow widths?

## Practical Design Rules For Agents And Developers

- Preserve the existing visual language unless the task is explicitly a redesign.
- Use tokens first, component CSS second, Tailwind last.
- Keep desktop polish high: spacing, motion, and hover states matter in this app.
- Prefer extending an existing primitive over adding a new one-off style island.
- If you discover a subtle visual rule while fixing a bug, document it near the code or in the relevant guide.
