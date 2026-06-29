# User bubble collapses to one character per row

**Date:** 2026-06-20  
**Component:** `cometline/src/lib/components/ChatThread.svelte` (not Composer)  
**Introduced by:** `484ddb7` — _style(ChatThread): enhance layout and spacing for user messages and actions_

## Summary

User message bubbles in the chat thread sometimes rendered as extremely narrow columns (~49px wide), stacking text vertically with one character per line. Affected both CJK (`你寫好了？`, `現在呢`) and Latin (`hihi`) short messages. Assistant bubbles on the same thread were unaffected.

## Symptoms

- `.bubble.user-bubble` computed to ~49×130px in DevTools (one glyph + horizontal padding).
- Text wrapped after every character instead of flowing horizontally.
- Bug appeared intermittently from a layout perspective but was reproducible on short user messages after the `user-stack` wrapper landed.
- Refreshing the page did not help until the structural layout fix was applied.

## Root cause

Commit `484ddb7` wrapped each user message in a new `.user-stack` flex column so Copy actions could sit below the bubble. That broke a layout invariant that assistant rows already satisfied.

**Assistant row (works):**

```
[avatar — fixed width][gap][assistant-stack — definite available width]
                              └── assistant-bubble: width fit-content; max-width 100%
```

The avatar (or `avatar-gutter`) gives the content column a definite horizontal size. `assistant-bubble` sizes to its text against that column.

**User row (broken after `484ddb7`):**

```
[user-stack only — width derived from children]
  └── user-bubble — width depends on parent
        └── .markdown.user-text
```

Circular sizing: parent width comes from child, child width comes from parent. With `.user-stack { min-width: 0 }` on a flex child, the stack could collapse to **min-content**.

Two CSS properties made min-content a single character wide:

1. **Inherited `word-break: break-word`** from `.bubble` (equivalent to allowing breaks anywhere for min-content sizing).
2. **`overflow-wrap: anywhere`** on `.markdown` (overridden later for user text, but the structural issue remained).

`width: fit-content` / `width: max-content` on `.user-bubble` alone did not fix this — the parent still had no definite width reference.

## Failed fixes (first pass)

| Attempt                                                         | Why it wasn't enough                                                      |
| --------------------------------------------------------------- | ------------------------------------------------------------------------- |
| `width: fit-content` on `.user-bubble`                          | Flex circular dependency still collapsed to min-content                   |
| `width: max-content` on `.user-bubble` + `.user-stack`          | Same — no definite column width from a sibling                            |
| `overflow-wrap: break-word` + `word-break: normal` on user text | Correct for text wrapping but didn't break the flex cycle                 |
| Suspecting `Composer.svelte`                                    | Composer only handles input; thread bubbles render in `ChatThread.svelte` |

## Fix

Mirror the assistant row structure so user messages get the same definite content-column width.

### 1. Structural — `ChatThread.svelte` markup

Add `avatar-gutter` to every user row (same spacer assistant/tool rows use):

```svelte
<div class="row user-row gap-2.5 md:gap-3 lg:gap-4">
	<div
		class="avatar-gutter size-9 shrink-0 md:size-10 lg:size-11 xl:size-12"
		aria-hidden="true"
	></div>
	<div class="user-stack">…</div>
</div>
```

### 2. Layout — `ChatThread.svelte` styles

```css
.user-row {
	justify-content: flex-start; /* was flex-end */
}

.user-stack {
	flex: 1 1 auto; /* fill content column beside gutter */
	min-width: 0;
	max-width: var(--chat-assistant-column);
	align-items: flex-end; /* right-align bubble within column */
}

.user-bubble {
	width: fit-content; /* match .assistant-bubble */
	max-width: 100%;
	word-break: normal;
}
```

### 3. Text wrapping — `AssistantMarkdown.svelte`

```css
.markdown.user-text {
	white-space: pre-wrap;
	overflow-wrap: break-word;
	word-break: normal;
}
```

### 4. Flight particle — `UserBubbleFlight.svelte`

Keep `width: fit-content` and `word-break: normal` on `.user-flight` so the send animation matches the final bubble.

## Prevention

- When changing chat row layout, keep **user and assistant rows structurally symmetric**: both need a fixed-width leading column (`avatar` / `avatar-gutter`) plus a flex content stack.
- Prefer `max-width: 100%` on bubbles relative to a **definite** parent column, not `max-width: var(--chat-content-column)` on a shrink-to-fit element.
- Avoid `min-width: 0` on a flex child whose width is purely content-derived unless a sibling establishes the column width.
- Short CJK + short Latin strings are good manual regression checks for min-content collapse.

## Files changed

- `cometline/src/lib/components/ChatThread.svelte`
- `cometline/src/lib/components/AssistantMarkdown.svelte`
- `cometline/src/lib/components/UserBubbleFlight.svelte`

## Verification

1. Send short CJK (`你寫好了？`) and Latin (`hihi`) — single horizontal line each.
2. Send a long paragraph — wraps within the content column max width.
3. Message with image + text — image grid unchanged.
4. User bubble flight animation — no width jump on handoff.
5. User bubbles right-align with the assistant content column.
