# Cometline Styling Guide

This document describes how styling works in Cometline and when to use each approach.

## Three-layer model

```
app.css (:root tokens + @theme + global utilities)
    ↓ imported by +layout.svelte
    ↓ runtime overrides via hero-composer-appearance.ts
Components: semantic classes → scoped <style> using var(--*)
         + Tailwind utilities for simple layout (optional)
         + :global() for {@html} / Lucide / CodeMirror
```

### Layer 1 — Global tokens (`src/app.css`)

Design tokens live in `:root` as CSS custom properties. Use these for any color, spacing, or motion value shared across two or more components.

```css
color: var(--text-muted);
transition: transform var(--duration-fast) var(--ease-smooth);
```

Tailwind v4 bridges a subset of tokens via `@theme inline`, exposing them as utility classes:

```html
<span class="text-text-muted">...</span>
```

Prefer `var(--*)` in scoped CSS; use Tailwind token classes in markup when applying simple utilities.

### Layer 2 — Scoped component CSS (`<style>` in `.svelte` files)

The default approach for component-specific layout, pseudo-elements, animations, and state selectors:

```svelte
<div class="bubble">...</div>

<style>
	.bubble {
		border-radius: var(--radius-card);
		background: var(--panel-bg);
	}
</style>
```

Svelte scopes these rules automatically. Do not extract to separate `.css` files unless creating a child component.

### Layer 3 — Tailwind utilities (markup)

Use for simple layout and spacing in HTML when it keeps markup readable:

```html
<div class="flex items-center gap-2">...</div>
```

**Do not mix** semantic classes and Tailwind utilities on the same element. Pick one approach per element.

## When to use what

| Approach           | When                                 | Example                                  |
| ------------------ | ------------------------------------ | ---------------------------------------- |
| `app.css` tokens   | Shared color/spacing/motion          | `var(--text-muted)`                      |
| Tailwind utilities | Simple layout in markup              | `flex gap-2 text-xs`                     |
| Scoped `<style>`   | Component layout, animations, states | `.composer.is-docked`                    |
| `:global()`        | Injected or third-party DOM only     | Markdown headings, Shiki blocks          |
| `style:` directive | Per-instance dynamic values          | `style:height`, `style:--thinking-color` |
| `@apply`           | **Never**                            | Anti-pattern in Tailwind v4              |

## Component size guidelines

| Scoped CSS lines | Action                               |
| ---------------- | ------------------------------------ |
| < 100            | Fine as-is                           |
| 100–200          | Acceptable; watch for duplication    |
| 200–400          | Consider extracting child components |
| 400+             | Should split                         |

Target for new components: **< 200 lines CSS, < 400 lines total**.

Exception: `AssistantMarkdown.svelte` may have a longer `:global()` block because it styles rendered HTML.

## Folder structure

```
src/lib/components/
├── chat/           ChatThread and thread UI pieces
├── composer/       Composer and input chrome
├── settings/       Settings modal and form primitives
└── ...             Top-level shell components (AppShell, Sidebar, etc.)
```

## Splitting components

When extracting a child component:

1. **Behavior unchanged** — refactor only, no UI changes
2. **Props down, events up** — data via props, callbacks for parent notification
3. **CSS follows markup** — move scoped rules with the extracted markup
4. **State ownership** — state used by one block moves to that child; shared state stays in parent
5. **Small PRs** — aim for < 500 lines diff per change

## Examples

Good small components to follow:

- `settings/SettingsToggle.svelte` — self-contained toggle with scoped styles
- `ThinkingIndicator.svelte` — dynamic values via `style:` directive

## Global utilities in `app.css`

| Class                    | Purpose                              |
| ------------------------ | ------------------------------------ |
| `.spin`                  | Loading spinner animation            |
| `.spin.small`            | 14×14 spinner variant                |
| `.content-panel-surface` | Panel border/shadow surface          |
| `.pane-focus-active`     | Focused pane highlight               |
| `.no-drag`               | Electron window drag region override |
